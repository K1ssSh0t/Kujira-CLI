package extractors

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
)

// Expresiones regulares precompiladas para mejor rendimiento
var (
	reScriptTag   = regexp.MustCompile(`<script[^>]*>([\s\S]*?)</script>`)
	reInnerHTML   = regexp.MustCompile(`innerHTML\s*=\s*(.+);`)
	reSubstring   = regexp.MustCompile(`^\s*\(\s*['"](.*?)['"]\s*\)\s*\.substring\s*\(\s*(\d+)\s*\)\s*$`)
	reParenString = regexp.MustCompile(`^\s*\(\s*['"](.*?)['"]\s*\)\s*$`)
)

func ExtractStreamTapeUrl(pageUrl string) (string, error) {
	// Realizar la petición HTTP
	resp, err := http.Get(pageUrl)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	// Leer el contenido HTML
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	html := string(body)

	// Buscar en los scripts la línea con 'botlink'
	jsLine, found := findScriptLineWithBotlink(html)
	if !found {
		return "", errors.New("no se encontró el script con 'botlink'")
	}

	// Extraer la expresión JavaScript
	expr, err := extractJSExpression(jsLine)
	if err != nil {
		return "", fmt.Errorf("no se pudo extraer la expresión JS: %w", err)
	}

	// Parsear la expresión para construir la URL
	resultUrl, err := parseJSExpression(expr)
	if err != nil {
		return "", fmt.Errorf("no se pudo evaluar la URL: %w", err)
	}

	return "https:" + resultUrl + "&stream=1", nil
}

func findScriptLineWithBotlink(html string) (string, bool) {
	scriptMatches := reScriptTag.FindAllStringSubmatch(html, -1)

	for _, match := range scriptMatches {
		if len(match) < 2 {
			continue
		}
		scriptContent := match[1]
		lines := strings.Split(scriptContent, "\n")

		for _, line := range lines {
			if strings.Contains(line, "botlink').innerHTML") {
				return strings.TrimSpace(line), true
			}
		}
	}
	return "", false
}

func extractJSExpression(jsLine string) (string, error) {
	match := reInnerHTML.FindStringSubmatch(jsLine)
	if len(match) < 2 {
		return "", errors.New("expresión JS no encontrada")
	}
	return match[1], nil
}

func parseJSExpression(expr string) (string, error) {
	parts := strings.Split(expr, "+")
	var result strings.Builder

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		switch {
		// Manejar literales de cadena (ej: "texto" o 'texto')
		case isStringLiteral(part):
			result.WriteString(part[1 : len(part)-1])

		// Manejar expresiones .substring() (ej: ('texto').substring(3))
		case reSubstring.MatchString(part):
			matches := reSubstring.FindStringSubmatch(part)
			str := matches[1]
			start, _ := strconv.Atoi(matches[2])

			if start > len(str) {
				return "", fmt.Errorf("índice substring inválido: %d", start)
			}
			result.WriteString(str[start:])

		// Manejar cadenas entre paréntesis (ej: ('texto'))
		case reParenString.MatchString(part):
			matches := reParenString.FindStringSubmatch(part)
			result.WriteString(matches[1])

		default:
			return "", fmt.Errorf("expresión no reconocida: %s", part)
		}
	}
	return result.String(), nil
}

func isStringLiteral(s string) bool {
	return (s[0] == '\'' && s[len(s)-1] == '\'') ||
		(s[0] == '"' && s[len(s)-1] == '"')
}

// // Ejemplo de uso:
// func main() {
// 	url, err := extractStreamTapeUrl("https://streamtape.com/e/mPgZzwzJyOHbrPr/")
// 	if err != nil {
// 		fmt.Println("Error:", err)
// 		return
// 	}
// 	fmt.Println("URL resultante:", url)
// }
