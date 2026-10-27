package compat

import "testing"

// TestAngular20Node2212OK vérifie qu'Angular 20 accepte Node 22.12.
func TestAngular20Node2212OK(t *testing.T) {
	if err := AngularNode(20, "22.12.0"); err != nil {
		t.Fatalf("attendu compatible, obtenu : %v", err)
	}
}

// TestAngular20Node18Rejected vérifie qu'Angular 20 refuse Node 18.
func TestAngular20Node18Rejected(t *testing.T) {
	err := AngularNode(20, "18.20.0")
	if err == nil {
		t.Fatal("attendu incompatible, obtenu nil")
	}
	want := "Angular 20 exige Node ^20.19 || ^22.12 || ^24"
	if err.Error() != want {
		t.Fatalf("message attendu %q, obtenu %q", want, err.Error())
	}
}

// TestUnknownAngularMajorAllowed vérifie qu'un majeur Angular inconnu ne
// bloque pas.
func TestUnknownAngularMajorAllowed(t *testing.T) {
	if err := AngularNode(99, "0.0.1"); err != nil {
		t.Fatalf("attendu nil pour majeur inconnu, obtenu : %v", err)
	}
}