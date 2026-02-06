// Package site provides site resolution and enrichment for alerts.
package site

import (
	"regexp"
	"strings"
)

// hostnamePattern represents a pattern for extracting site codes from hostnames.
type hostnamePattern struct {
	regex       *regexp.Regexp
	description string
}

// patterns defines the hostname patterns for site code extraction.
// Order matters - first match wins.
var patterns = []hostnamePattern{
	{
		// dfw1-router01 -> dfw1
		regex:       regexp.MustCompile(`^([a-z]{2,4}\d+)-`),
		description: "prefix with hyphen (e.g., dfw1-router01)",
	},
	{
		// core-nyc2-sw01 -> nyc2
		regex:       regexp.MustCompile(`^[a-z]+-([a-z]{2,4}\d+)-`),
		description: "middle segment with hyphens (e.g., core-nyc2-sw01)",
	},
	{
		// server.lax1.example.com -> lax1
		regex:       regexp.MustCompile(`\.([a-z]{2,4}\d+)\.`),
		description: "domain segment (e.g., server.lax1.example.com)",
	},
	{
		// router-ord3 -> ord3
		regex:       regexp.MustCompile(`-([a-z]{2,4}\d+)$`),
		description: "suffix with hyphen (e.g., router-ord3)",
	},
	{
		// alt format: NYC-DC1-server01 -> nyc-dc1 (normalize to lowercase)
		regex:       regexp.MustCompile(`^([A-Za-z]{2,4}-[A-Za-z]{2,4}\d*)-`),
		description: "compound prefix (e.g., NYC-DC1-server01)",
	},
	{
		// FQDN pattern: host.datacenter.region.example.com
		regex:       regexp.MustCompile(`^[^.]+\.([a-z]{2,4}\d*)\.[^.]+\.[^.]+$`),
		description: "second-level domain segment",
	},
}

// ExtractSiteCodeFromHostname attempts to extract a site code from a hostname.
// Returns the extracted site code (lowercase) and a boolean indicating success.
func ExtractSiteCodeFromHostname(hostname string) (string, bool) {
	if hostname == "" {
		return "", false
	}

	// Normalize to lowercase for matching
	normalized := strings.ToLower(hostname)

	for _, p := range patterns {
		matches := p.regex.FindStringSubmatch(normalized)
		if len(matches) >= 2 {
			return matches[1], true
		}
	}

	return "", false
}

// ExtractSiteCodeFromInstance extracts a site code from an instance identifier.
// Instance can be a hostname, IP:port, or other identifier.
func ExtractSiteCodeFromInstance(instance string) (string, bool) {
	if instance == "" {
		return "", false
	}

	// Remove port if present
	hostname := instance
	if colonIdx := strings.LastIndex(instance, ":"); colonIdx != -1 {
		// Check if it's an IP:port or hostname:port
		hostname = instance[:colonIdx]
	}

	// Skip IP addresses
	if isIPAddress(hostname) {
		return "", false
	}

	return ExtractSiteCodeFromHostname(hostname)
}

// isIPAddress checks if a string looks like an IP address.
func isIPAddress(s string) bool {
	// Simple check for IPv4
	parts := strings.Split(s, ".")
	if len(parts) == 4 {
		for _, p := range parts {
			if len(p) > 3 || len(p) == 0 {
				return false
			}
			for _, c := range p {
				if c < '0' || c > '9' {
					return false
				}
			}
		}
		return true
	}

	// Check for IPv6 (contains colons but no letters typical of hostnames)
	if strings.Contains(s, ":") && !strings.ContainsAny(s, "ghijklmnopqrstuvwxyzGHIJKLMNOPQRSTUVWXYZ") {
		return true
	}

	return false
}

// NormalizeSiteCode normalizes a site code to a consistent format.
// Converts to lowercase and removes extra whitespace.
func NormalizeSiteCode(code string) string {
	return strings.TrimSpace(strings.ToLower(code))
}
