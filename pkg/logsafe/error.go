package logsafe

import (
	"os/exec"
	"reflect"
	"strconv"
	"strings"

	"github.com/go-sql-driver/mysql"
)

// Error preserves the chain in memory while excluding sensitive provider bodies and SQL values from output.
func Error(err error) string {
	if err == nil {
		return ""
	}

	if safe, ok := err.(interface{ LogMessage() string }); ok {
		return safe.LogMessage()
	}
	//nolint:errorlint // Each chain node is sanitized in place; flattening with As would lose outer operations.
	if driver, ok := err.(*mysql.MySQLError); ok {
		return "mysql error " + strconv.Itoa(int(driver.Number))
	}
	//nolint:errorlint // Preserve this exact process failure node and its captured stderr before recursing.
	if process, ok := err.(*exec.ExitError); ok && len(process.Stderr) > 0 {
		return process.Error() + ": " + string(process.Stderr)
	}

	text := err.Error()
	if many, ok := err.(interface{ Unwrap() []error }); ok {
		for _, child := range many.Unwrap() {
			if child != nil {
				text = strings.ReplaceAll(text, child.Error(), Error(child))
			}
		}
	} else if one, ok := err.(interface{ Unwrap() error }); ok {
		if child := one.Unwrap(); child != nil {
			text = strings.ReplaceAll(text, child.Error(), Error(child))
		}
	}

	return text
}

// Secrets extracts configured credentials by field identity, without exposing a config dump.
func Secrets(config any) []string {
	var (
		out   []string
		visit func(reflect.Value, string)
	)

	visit = func(v reflect.Value, name string) {
		if !v.IsValid() {
			return
		}

		if v.Kind() == reflect.Pointer || v.Kind() == reflect.Interface {
			if !v.IsNil() {
				visit(v.Elem(), name)
			}

			return
		}

		// Reflection only descends into configuration structs and inspects string fields.
		//exhaustive:ignore
		switch v.Kind() {
		case reflect.Struct:
			for i := range v.NumField() {
				if v.Type().Field(i).IsExported() {
					visit(v.Field(i), v.Type().Field(i).Name)
				}
			}
		case reflect.String:
			lower := strings.ToLower(name)
			if secretsCondition(lower) {
				out = append(out, v.String())
			}
		}
	}
	visit(reflect.ValueOf(config), "")

	return out
}

func secretsCondition(lower string) bool {
	return strings.Contains(lower, "secret") || strings.Contains(lower, "password") || strings.Contains(lower, "apikey") || lower == "authcode" || lower == "appkey"
}
