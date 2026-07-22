package utils

import (
	"strings"
	"testing"
)

type hashSample struct {
	ID   uint   `gorm:"primarykey"`
	Name string `gorm:"size:64;comment:名称"`
}

type hashBase struct {
	ID uint `gorm:"primarykey"`
}

type hashSampleEmbedded struct {
	hashBase
	Code string `gorm:"size:32;index"`
}

func TestModelSchemaHashStable(t *testing.T) {
	h1 := ModelSchemaHash(&hashSample{Name: "x"})
	h2 := ModelSchemaHash(&hashSample{})
	if h1 != h2 {
		t.Fatalf("hash should ignore instance values: %s vs %s", h1, h2)
	}
}

func TestModelSchemaHashDetectsTagChange(t *testing.T) {
	type a struct {
		Name string `gorm:"size:64"`
	}
	type b struct {
		Name string `gorm:"size:128"`
	}
	if ModelSchemaHash(&a{}) == ModelSchemaHash(&b{}) {
		t.Fatal("expected different hash for different gorm tags")
	}
}

func TestModelSchemaHashIndependent(t *testing.T) {
	type foo struct {
		A string `gorm:"size:8"`
	}
	type bar struct {
		B string `gorm:"size:8"`
	}
	if ModelSchemaHash(&foo{}) == ModelSchemaHash(&bar{}) {
		t.Fatal("different models should have different hashes")
	}
}

func TestModelSchemaHashEmbedded(t *testing.T) {
	h := ModelSchemaHash(&hashSampleEmbedded{})
	if h == "" {
		t.Fatal("empty hash")
	}
	type onlyBase struct {
		hashBase
	}
	if ModelSchemaHash(&hashSampleEmbedded{}) == ModelSchemaHash(&onlyBase{}) {
		t.Fatal("expected different hash when extra fields present")
	}
}

func TestModelTypeName(t *testing.T) {
	name := ModelTypeName(&hashSample{})
	if !strings.Contains(name, "hashSample") {
		t.Fatalf("unexpected type name: %q", name)
	}
}
