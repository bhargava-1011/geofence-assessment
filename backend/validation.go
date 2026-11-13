package main

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// validateCoordinates checks a polygon given as [latitude, longitude] pairs.
func validateCoordinates(c [][]float64) error {
	if len(c) < 4 {
		return errors.New("coordinates must contain at least 4 points (3 unique points plus the closing point)")
	}
	for i, p := range c {
		if len(p) != 2 {
			return fmt.Errorf("coordinates[%d] must be a [latitude, longitude] pair", i)
		}
		if p[0] < -90 || p[0] > 90 {
			return fmt.Errorf("coordinates[%d]: latitude must be between -90 and 90", i)
		}
		if p[1] < -180 || p[1] > 180 {
			return fmt.Errorf("coordinates[%d]: longitude must be between -180 and 180", i)
		}
	}
	first, last := c[0], c[len(c)-1]
	if first[0] != last[0] || first[1] != last[1] {
		return errors.New("polygon must be closed: first and last coordinates must be identical")
	}
	unique := map[[2]float64]bool{}
	for _, p := range c[:len(c)-1] {
		unique[[2]float64{p[0], p[1]}] = true
	}
	if len(unique) < 3 {
		return errors.New("polygon needs at least 3 unique points")
	}
	return nil
}

// polygonWKT converts [lat, lng] pairs to WKT, which uses (lng lat) order.
func polygonWKT(c [][]float64) string {
	parts := make([]string, len(c))
	for i, p := range c {
		parts[i] = strconv.FormatFloat(p[1], 'f', -1, 64) + " " + strconv.FormatFloat(p[0], 'f', -1, 64)
	}
	return "POLYGON((" + strings.Join(parts, ",") + "))"
}
