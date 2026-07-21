package config

import "testing"

func TestDevelopmentDisablesAuth(t *testing.T) {
	t.Setenv("TIKAWANG_ENV", "development")
	if Load().AuthRequired {
		t.Fatal("development environment should not require authentication")
	}
}

func TestProductionRequiresAuth(t *testing.T) {
	t.Setenv("TIKAWANG_ENV", "production")
	if !Load().AuthRequired {
		t.Fatal("production environment should require authentication")
	}
}

func TestUnknownEnvironmentRequiresAuth(t *testing.T) {
	t.Setenv("TIKAWANG_ENV", "")
	if !Load().AuthRequired {
		t.Fatal("authentication should be required by default")
	}
}
