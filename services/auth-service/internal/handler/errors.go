package handler

import (
	"fmt"

	"github.com/go-playground/validator/v10"
)

// formatValidationError mengkonversi validation error dari go-playground/validator
// menjadi pesan error yang user-friendly dalam Bahasa Indonesia
//
// Supported validation tags:
//   - required: "X harus diisi"
//   - email: "X harus berupa alamat email yang valid"
//   - min: "X minimal Y karakter"
//   - max: "X maksimal Y karakter"
//   - alphanum: "X hanya boleh berisi huruf dan angka"
//   - numeric: "X harus berupa angka"
//   - oneof: "X memiliki nilai yang tidak valid"
func FormatValidationError(err error) string {
	// Cek apakah error adalah validator.ValidationErrors
	if validationErrs, ok := err.(validator.ValidationErrors); ok {
		// Ambil error pertama (biasanya yang paling relevan)
		if len(validationErrs) > 0 {
			fieldErr := validationErrs[0]
			field := fieldErr.Field()
			tag := fieldErr.Tag()

			// Translate nama field ke Bahasa Indonesia
			fieldName := translateFieldName(field)

			// Format pesan berdasarkan validation tag
			switch tag {
			case "required":
				return fmt.Sprintf("%s harus diisi", fieldName)
			case "email":
				return fmt.Sprintf("%s harus berupa alamat email yang valid", fieldName)
			case "min":
				param := fieldErr.Param()
				switch fieldErr.Kind().String() {
				case "string":
					return fmt.Sprintf("%s minimal %s karakter", fieldName, param)
				default:
					return fmt.Sprintf("%s minimal %s", fieldName, param)
				}
			case "max":
				param := fieldErr.Param()
				switch fieldErr.Kind().String() {
				case "string":
					return fmt.Sprintf("%s maksimal %s karakter", fieldName, param)
				default:
					return fmt.Sprintf("%s maksimal %s", fieldName, param)
				}
			case "alphanum":
				return fmt.Sprintf("%s hanya boleh berisi huruf dan angka", fieldName)
			case "numeric":
				return fmt.Sprintf("%s harus berupa angka", fieldName)
			case "oneof":
				return fmt.Sprintf("%s memiliki nilai yang tidak valid", fieldName)
			default:
				return fmt.Sprintf("Validasi gagal untuk field %s", fieldName)
			}
		}
	}

	// Fallback: return error asli jika bukan validation error
	return err.Error()
}

// translateFieldName menerjemahkan nama field dari struct ke Bahasa Indonesia
// Jika field tidak ada di mapping, return nama field asli
func translateFieldName(field string) string {
	translations := map[string]string{
		// Auth fields
		"Email":           "Email",
		"Username":        "Username",
		"Password":        "Password",
		"FullName":        "Nama Lengkap",
		"CurrentPassword": "Password Saat Ini",
		"NewPassword":     "Password Baru",

		// Role fields
		"RoleName":    "Nama Role",
		"Description": "Deskripsi",
		"UserID":      "User ID",
		"RoleID":      "Role ID",
		"Permissions": "Permissions",

		// Common fields
		"Limit":  "Limit",
		"Offset": "Offset",
		"Page":   "Halaman",
	}

	if translated, ok := translations[field]; ok {
		return translated
	}

	// Field tidak ada di mapping, return nama asli
	return field
}
