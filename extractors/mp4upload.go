package extractors

import (
	"fmt"
	"io"
	"net/http"
	"regexp"
)

// UserAgent es el User-Agent usado en todas las peticiones de extracción y que
// conviene reenviar al reproductor (mpv/vlc).
const UserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/58.0.3029.110 Safari/537.3"

var (
	reMp4UploadID  = regexp.MustCompile(`mp4upload\.com\/(?:embed-)?([A-Za-z0-9]+)`)
	reMp4UploadSrc = regexp.MustCompile(`(?s)player\.src\(\{.*?src:\s*"([^"]+?)"`)
)

// ExtractMp4UploadUrl extrae la URL directa de un embed de Mp4Upload.
// Devuelve la URL directa del video y el Referer que el reproductor DEBE enviar
// (la propia URL embed; sin él, el CDN responde 403).
func ExtractMp4UploadUrl(pageURL, referer string) (string, string, error) {
	// 1-2) Normalizar la URL embed a partir del ID.
	realURL := pageURL
	if m := reMp4UploadID.FindStringSubmatch(pageURL); len(m) > 1 {
		realURL = fmt.Sprintf("https://www.mp4upload.com/embed-%s.html", m[1])
	}

	// 3) GET con User-Agent + Referer opcional.
	req, err := http.NewRequest("GET", realURL, nil)
	if err != nil {
		return "", "", err
	}
	req.Header.Set("User-Agent", UserAgent)
	if referer != "" {
		req.Header.Set("Referer", referer)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", "", err
	}
	html := string(body)

	// 4) Buscar la fuente en el HTML.
	m := reMp4UploadSrc.FindStringSubmatch(html)
	if len(m) < 2 {
		return "", "", fmt.Errorf("mp4upload: no se encontró player.src (status %d, %d bytes)", resp.StatusCode, len(html))
	}

	// 5) El Referer a devolver es la URL embed.
	return m[1], realURL, nil
}
