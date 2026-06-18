package extractors

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

var (
	rePacked       = regexp.MustCompile(`(?s)\}\s*\('([\s\S]*)',\s*(.*?),\s*(\d+),\s*'([^']*)'\.split\('\|'\)`)
	rePackedWord   = regexp.MustCompile(`\b[a-zA-Z0-9_]+\b`)
	reUqloadFile   = regexp.MustCompile(`(?s)file:\s*"([^"]+)"[^}]*?\}`)
	reUqloadM3u8   = regexp.MustCompile(`https?://[^"'\s]+\.m3u8[^"'\s]*`)
	unbaseAlphabet = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	unbaseAlphaBig = " !\"#$%&'()*+,-./0123456789:;<=>?@ABCDEFGHIJKLMNOPQRSTUVWXYZ[\\]^_`abcdefghijklmnopqrstuvwxyz{|}~"
)

// unbase convierte una palabra en su índice numérico según el radix dado,
// replicando la función `unbase` del desempaquetador de P.A.C.K.E.R.
func unbase(str string, radix int) int {
	if radix <= 36 {
		n, err := strconv.ParseInt(str, radix, 64)
		if err != nil {
			return 0
		}
		return int(n)
	}

	var alphabet string
	if radix <= 62 {
		alphabet = unbaseAlphabet[:radix]
	} else {
		alphabet = unbaseAlphaBig[:radix]
	}
	dict := make(map[rune]int, len(alphabet))
	for i, ch := range alphabet {
		dict[ch] = i
	}

	runes := []rune(str)
	value := 0
	pow := 1
	// dígitos invertidos: el primer carácter es el menos significativo tras invertir.
	for i := len(runes) - 1; i >= 0; i-- {
		value += dict[runes[i]] * pow
		pow *= radix
	}
	return value
}

// unpackPacked desempaqueta JS comprimido con P.A.C.K.E.R. (eval(function(p,a,c,k,e,...))).
func unpackPacked(js string) (string, error) {
	m := rePacked.FindStringSubmatch(js)
	if m == nil {
		return "", fmt.Errorf("uqload: no se encontró bloque P.A.C.K.E.R.")
	}
	payload := strings.ReplaceAll(m[1], `\'`, `'`)
	radix, err := strconv.Atoi(strings.TrimSpace(m[2]))
	if err != nil || radix == 0 {
		radix = 36
	}
	count, _ := strconv.Atoi(m[3])
	symtab := strings.Split(m[4], "|")
	if len(symtab) != count {
		return "", fmt.Errorf("uqload: symtab (%d) != count (%d)", len(symtab), count)
	}

	unpacked := rePackedWord.ReplaceAllStringFunc(payload, func(word string) string {
		x := unbase(word, radix)
		if x >= 0 && x < len(symtab) && symtab[x] != "" {
			return symtab[x]
		}
		return word
	})
	return unpacked, nil
}

// ExtractUqloadUrl extrae la URL directa (mp4 o m3u8) de un embed de Uqload.
// Devuelve la URL del video y el Referer que el reproductor DEBE enviar
// (la propia URL embed).
func ExtractUqloadUrl(pageURL, referer string) (string, string, error) {
	// 1) GET de la pageURL para "calentar" la sesión.
	if _, err := uqloadGet(pageURL, referer); err != nil {
		return "", "", err
	}

	parsed, err := url.Parse(pageURL)
	if err != nil {
		return "", "", err
	}

	// 2) Extraer file_code de la ruta.
	path := strings.TrimSuffix(parsed.Path, ".html")
	segments := strings.Split(path, "/")
	last := segments[len(segments)-1]
	parts := strings.Split(last, "-")
	fileCode := parts[len(parts)-1]
	if fileCode == "" {
		return "", "", fmt.Errorf("uqload: no se pudo extraer file_code de %s", pageURL)
	}

	// 3) POST a {scheme}://{host}/dl.
	dlURL := fmt.Sprintf("%s://%s/dl", parsed.Scheme, parsed.Host)
	form := url.Values{
		"op":        {"embed"},
		"file_code": {fileCode},
		"auto":      {"1"},
		"referer":   {""},
	}
	req, err := http.NewRequest("POST", dlURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", UserAgent)
	req.Header.Set("Referer", pageURL)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", "", err
	}
	dlHTML := string(body)

	// 4) Desempaquetar P.A.C.K.E.R.
	unpacked, err := unpackPacked(dlHTML)
	if err != nil {
		return "", "", fmt.Errorf("%w (status %d, %d bytes)", err, resp.StatusCode, len(dlHTML))
	}

	// 5) Buscar la URL del video.
	if m := reUqloadFile.FindStringSubmatch(unpacked); len(m) > 1 {
		return m[1], pageURL, nil
	}
	if m := reUqloadM3u8.FindString(unpacked); m != "" {
		return m, pageURL, nil
	}

	return "", "", fmt.Errorf("uqload: no se encontró URL de video en el JS desempaquetado")
}

func uqloadGet(rawURL, referer string) (string, error) {
	req, err := http.NewRequest("GET", rawURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", UserAgent)
	if referer != "" {
		req.Header.Set("Referer", referer)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	return string(body), err
}
