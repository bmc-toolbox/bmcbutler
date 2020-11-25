package internal

import "unicode"

// IsntLetterOrNumber check if the give rune is not a letter nor a number
func IsntLetterOrNumber(c rune) bool {
	return !unicode.IsLetter(c) && !unicode.IsNumber(c)
}

func ErrStringOrEmpty(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
