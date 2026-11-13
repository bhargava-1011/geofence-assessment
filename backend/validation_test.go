package main

import "testing"

func TestValidateCoordinates(t *testing.T) {
	square := [][]float64{{37.77, -122.41}, {37.78, -122.41}, {37.78, -122.40}, {37.77, -122.40}, {37.77, -122.41}}
	tests := []struct {
		name    string
		coords  [][]float64
		wantErr bool
	}{
		{"valid square", square, false},
		{"too few points", square[:3], true},
		{"not closed", [][]float64{{1, 1}, {2, 2}, {3, 3}, {4, 4}}, true},
		{"bad latitude", [][]float64{{91, 0}, {1, 1}, {2, 2}, {91, 0}}, true},
		{"bad longitude", [][]float64{{0, 181}, {1, 1}, {2, 2}, {0, 181}}, true},
		{"wrong pair length", [][]float64{{1, 1, 1}, {1, 2}, {2, 2}, {1, 1, 1}}, true},
		{"duplicate points only", [][]float64{{1, 1}, {1, 1}, {2, 2}, {1, 1}}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := validateCoordinates(tt.coords); (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestPolygonWKTSwapsToLngLat(t *testing.T) {
	got := polygonWKT([][]float64{{10, 20}, {11, 21}, {12, 22}, {10, 20}})
	want := "POLYGON((20 10,21 11,22 12,20 10))"
	if got != want {
		t.Fatalf("got %s want %s", got, want)
	}
}
