package references

import (
	"fmt"
	"regexp"
	"strings"
)

var outputPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

func Expression(resourceName, output string) (string, error) {
	resourceName = strings.TrimSpace(resourceName)
	output = strings.TrimSpace(output)
	if resourceName == "" {
		return "", fmt.Errorf("Railway reference resource name must not be empty")
	}
	if strings.ContainsAny(resourceName, "${}\r\n") {
		return "", fmt.Errorf("Railway reference resource name contains reserved characters")
	}
	if !outputPattern.MatchString(output) {
		return "", fmt.Errorf("invalid Railway reference output %q", output)
	}
	return "${{" + resourceName + "." + output + "}}", nil
}

func MustExpression(resourceName, output string) string {
	value, err := Expression(resourceName, output)
	if err != nil {
		panic(err)
	}
	return value
}

// HCLLiteral escapes a Railway reference for use as a literal inside an HCL
// quoted string. Terraform evaluates "$${" to a literal "${".
func HCLLiteral(expression string) string {
	return strings.ReplaceAll(expression, "${", "$${")
}

func Bucket(resourceName string) map[string]string {
	outputs := []string{
		"BUCKET",
		"ENDPOINT",
		"ACCESS_KEY_ID",
		"SECRET_ACCESS_KEY",
		"REGION",
	}
	result := make(map[string]string, len(outputs))
	for _, output := range outputs {
		result[output] = MustExpression(resourceName, output)
	}
	return result
}

func Postgres(resourceName string) map[string]string {
	outputs := []string{
		"DATABASE_URL",
		"PGHOST",
		"PGPORT",
		"PGUSER",
		"PGPASSWORD",
		"PGDATABASE",
	}
	result := make(map[string]string, len(outputs))
	for _, output := range outputs {
		result[output] = MustExpression(resourceName, output)
	}
	return result
}
