package version

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Version represents a semantic version
type Version struct {
	Major int
	Minor int
	Patch int
}

// ParseVersion parses a semantic version string like "v1.2.3" or "1.2.3"
func ParseVersion(versionStr string) (*Version, error) {
	// Remove 'v' prefix if present
	versionStr = strings.TrimPrefix(versionStr, "v")

	// Match semantic version pattern
	re := regexp.MustCompile(`^(\d+)\.(\d+)\.(\d+)$`)
	matches := re.FindStringSubmatch(versionStr)
	if len(matches) != 4 {
		return nil, fmt.Errorf("invalid version format '%s', expected 'major.minor.patch'", versionStr)
	}

	major, err := strconv.Atoi(matches[1])
	if err != nil {
		return nil, fmt.Errorf("invalid major version: %w", err)
	}

	minor, err := strconv.Atoi(matches[2])
	if err != nil {
		return nil, fmt.Errorf("invalid minor version: %w", err)
	}

	patch, err := strconv.Atoi(matches[3])
	if err != nil {
		return nil, fmt.Errorf("invalid patch version: %w", err)
	}

	return &Version{
		Major: major,
		Minor: minor,
		Patch: patch,
	}, nil
}

// String returns the version as a string
func (v *Version) String() string {
	return fmt.Sprintf("v%d.%d.%d", v.Major, v.Minor, v.Patch)
}

// Compare compares two versions
// Returns: -1 if v < other, 0 if v == other, 1 if v > other
func (v *Version) Compare(other *Version) int {
	if v.Major != other.Major {
		if v.Major < other.Major {
			return -1
		}
		return 1
	}

	if v.Minor != other.Minor {
		if v.Minor < other.Minor {
			return -1
		}
		return 1
	}

	if v.Patch != other.Patch {
		if v.Patch < other.Patch {
			return -1
		}
		return 1
	}

	return 0
}

// IsCompatible checks if this version is compatible with a constraint
func (v *Version) IsCompatible(constraint *VersionConstraint) bool {
	return constraint.Satisfies(v)
}

// VersionConstraint represents a version constraint like ">=1.0.0" or "~1.2.0"
type VersionConstraint struct {
	Operator string   // ">=", "<=", ">", "<", "==", "~", "^"
	Version  *Version // The constraint version
}

// ParseVersionConstraint parses a version constraint string
func ParseVersionConstraint(constraintStr string) (*VersionConstraint, error) {
	// Handle different constraint operators
	operators := []string{">=", "<=", "==", "~", "^", ">", "<"}

	for _, op := range operators {
		if strings.HasPrefix(constraintStr, op) {
			versionStr := strings.TrimSpace(constraintStr[len(op):])
			version, err := ParseVersion(versionStr)
			if err != nil {
				return nil, fmt.Errorf("invalid version in constraint '%s': %w", constraintStr, err)
			}

			return &VersionConstraint{
				Operator: op,
				Version:  version,
			}, nil
		}
	}

	// If no operator, assume exact match
	version, err := ParseVersion(constraintStr)
	if err != nil {
		return nil, fmt.Errorf("invalid version constraint '%s': %w", constraintStr, err)
	}

	return &VersionConstraint{
		Operator: "==",
		Version:  version,
	}, nil
}

// Satisfies checks if a version satisfies this constraint
func (c *VersionConstraint) Satisfies(version *Version) bool {
	cmp := version.Compare(c.Version)

	switch c.Operator {
	case "==":
		return cmp == 0
	case ">":
		return cmp > 0
	case ">=":
		return cmp >= 0
	case "<":
		return cmp < 0
	case "<=":
		return cmp <= 0
	case "~":
		// Tilde allows patch-level changes: ~1.2.3 allows >=1.2.3 and <1.3.0
		if version.Major != c.Version.Major || version.Minor != c.Version.Minor {
			return false
		}
		return version.Patch >= c.Version.Patch
	case "^":
		// Caret allows minor-level changes: ^1.2.3 allows >=1.2.3 and <2.0.0
		if version.Major != c.Version.Major {
			return false
		}
		if version.Minor < c.Version.Minor {
			return false
		}
		if version.Minor == c.Version.Minor && version.Patch < c.Version.Patch {
			return false
		}
		return true
	default:
		return false
	}
}

// String returns the constraint as a string
func (c *VersionConstraint) String() string {
	return c.Operator + c.Version.String()
}

// ValidateVersionConstraints validates that a set of constraints are consistent
func ValidateVersionConstraints(constraints []*VersionConstraint) error {
	if len(constraints) <= 1 {
		return nil
	}

	// Check for contradictory constraints
	for i, c1 := range constraints {
		for j, c2 := range constraints {
			if i >= j {
				continue
			}

			// Check if the constraints can be satisfied simultaneously
			if !c1.IsCompatibleWith(c2) {
				return fmt.Errorf("contradictory version constraints: %s and %s", c1.String(), c2.String())
			}
		}
	}

	return nil
}

// IsCompatibleWith checks if two constraints can be satisfied simultaneously
func (c *VersionConstraint) IsCompatibleWith(other *VersionConstraint) bool {
	// This is a simplified check - a full implementation would be more complex
	// For now, we'll check some basic incompatible cases

	if c.Operator == "==" && other.Operator == "==" {
		return c.Version.Compare(other.Version) == 0
	}

	if c.Operator == ">" && other.Operator == "<" {
		return c.Version.Compare(other.Version) < 0
	}

	if c.Operator == "<" && other.Operator == ">" {
		return other.Version.Compare(c.Version) < 0
	}

	// For most other cases, assume they're compatible
	// A more sophisticated implementation would check all combinations
	return true
}

// FindBestVersion finds the best version from a list that satisfies all constraints
func FindBestVersion(versions []*Version, constraints []*VersionConstraint) (*Version, error) {
	if len(versions) == 0 {
		return nil, fmt.Errorf("no versions available")
	}

	// Validate constraints
	if err := ValidateVersionConstraints(constraints); err != nil {
		return nil, err
	}

	var bestVersion *Version
	for _, version := range versions {
		satisfiesAll := true
		for _, constraint := range constraints {
			if !constraint.Satisfies(version) {
				satisfiesAll = false
				break
			}
		}

		if satisfiesAll {
			if bestVersion == nil || version.Compare(bestVersion) > 0 {
				bestVersion = version
			}
		}
	}

	if bestVersion == nil {
		constraintStrs := make([]string, len(constraints))
		for i, c := range constraints {
			constraintStrs[i] = c.String()
		}
		return nil, fmt.Errorf("no version satisfies all constraints: %s", strings.Join(constraintStrs, ", "))
	}

	return bestVersion, nil
}
