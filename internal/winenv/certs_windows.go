//go:build windows

package winenv

import (
	"crypto/sha1"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// certAddEncodedCertificateToStore est l'adresse de CertAddEncodedCertificateToStore
// (crypt32.dll), absente de golang.org/x/sys/windows.
var certAddEncodedCertificateToStore = syscall.NewLazyDLL("crypt32.dll").NewProc("CertAddEncodedCertificateToStore")

// certStore implémente CertStore via le magasin de certificats utilisateur Windows.
type certStore struct{}

// NewCertStore retourne un CertStore basé sur le magasin de certificats utilisateur.
func NewCertStore() CertStore {
	return &certStore{}
}

// openUserRoot ouvre le magasin de certificats racine de l'utilisateur courant.
func openUserRoot() (windows.Handle, error) {
	store, err := windows.CertOpenSystemStore(0, windows.StringToUTF16Ptr("ROOT"))
	if err != nil {
		return 0, fmt.Errorf("ouverture du magasin de certificats utilisateur : %w", err)
	}
	return store, nil
}

// readPEMCert lit le premier bloc CERTIFICATE d'un fichier PEM et retourne son DER.
func readPEMCert(pemPath string) ([]byte, error) {
	data, err := os.ReadFile(pemPath)
	if err != nil {
		return nil, fmt.Errorf("lecture du certificat %s : %w", pemPath, err)
	}
	for {
		var block *pem.Block
		block, data = pem.Decode(data)
		if block == nil {
			return nil, fmt.Errorf("aucun bloc CERTIFICATE dans %s", pemPath)
		}
		if block.Type == "CERTIFICATE" {
			return block.Bytes, nil
		}
	}
}

// AddUserRoot ajoute un certificat racine au magasin utilisateur s'il est absent.
func (c *certStore) AddUserRoot(pemPath string) (bool, error) {
	der, err := readPEMCert(pemPath)
	if err != nil {
		return false, err
	}
	store, err := openUserRoot()
	if err != nil {
		return false, err
	}
	defer windows.CertCloseStore(store, 0)

	present, err := hasUserRootDER(store, der)
	if err != nil {
		return false, err
	}
	if present {
		return false, nil
	}

	added, _, err := certAddEncodedCertificateToStore.Call(
		uintptr(store),
		uintptr(windows.X509_ASN_ENCODING|windows.PKCS_7_ASN_ENCODING),
		uintptr(unsafe.Pointer(&der[0])),
		uintptr(len(der)),
		uintptr(windows.CERT_STORE_ADD_USE_EXISTING),
		0,
	)
	if added == 0 {
		return false, fmt.Errorf("ajout du certificat racine : %w", err)
	}
	return true, nil
}

// HasUserRoot indique si un certificat racine est présent dans le magasin utilisateur.
func (c *certStore) HasUserRoot(pemPath string) (bool, error) {
	der, err := readPEMCert(pemPath)
	if err != nil {
		return false, err
	}
	store, err := openUserRoot()
	if err != nil {
		return false, err
	}
	defer windows.CertCloseStore(store, 0)
	return hasUserRootDER(store, der)
}

// hasUserRootDER compare l'empreinte SHA-1 du DER aux certificats du magasin.
func hasUserRootDER(store windows.Handle, der []byte) (bool, error) {
	want := sha1.Sum(der)
	var prev *windows.CertContext
	for {
		ctx, err := windows.CertEnumCertificatesInStore(store, prev)
		if err != nil {
			if errors.Is(err, windows.ERROR_NO_MORE_FILES) {
				return false, nil
			}
			return false, fmt.Errorf("énumération des certificats : %w", err)
		}
		prev = ctx
		if ctx.EncodedCert == nil || ctx.Length == 0 {
			continue
		}
		raw := unsafe.Slice(ctx.EncodedCert, ctx.Length)
		if sha1.Sum(raw) == want {
			return true, nil
		}
	}
}
