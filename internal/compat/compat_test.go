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
	want := "Angular 20 exige Node ^20.19.0 || ^22.12.0 || >=24.0.0"
	if err.Error() != want {
		t.Fatalf("message attendu %q, obtenu %q", want, err.Error())
	}
}

// TestErrorMessageUsesExactRange vérifie que le message d'erreur reprend la
// plage exacte déclarée pour Angular 20.
func TestErrorMessageUsesExactRange(t *testing.T) {
	err := AngularNode(20, "18.0.0")
	if err == nil {
		t.Fatal("attendu incompatible, obtenu nil")
	}
	want := "Angular 20 exige Node ^20.19.0 || ^22.12.0 || >=24.0.0"
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

// TestAngular22Node26Allowed vérifie qu'Angular 22 accepte Node 26.1.0 via la
// plage >=26.0.0.
func TestAngular22Node26Allowed(t *testing.T) {
	if err := AngularNode(22, "26.1.0"); err != nil {
		t.Fatalf("attendu compatible, obtenu : %v", err)
	}
}

// TestAngular22Node2221Rejected vérifie qu'Angular 22 refuse Node 22.21.0
// (inférieur au minimum ^22.22.3).
func TestAngular22Node2221Rejected(t *testing.T) {
	if err := AngularNode(22, "22.21.0"); err == nil {
		t.Fatal("attendu incompatible, obtenu nil")
	}
}

// TestAngular20Node25Allowed vérifie qu'Angular 20 accepte Node 25.0.0 via la
// plage >=24.0.0.
func TestAngular20Node25Allowed(t *testing.T) {
	if err := AngularNode(20, "25.0.0"); err != nil {
		t.Fatalf("attendu compatible, obtenu : %v", err)
	}
}