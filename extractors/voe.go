package extractors

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/grafov/m3u8"
)

// SubtitleFile representa un archivo de subtítulos
type SubtitleFile struct {
	URL      string
	Language string
}

// ExtractorLink contiene información de un link extraído
type ExtractorLink struct {
	Source  string
	Name    string
	URL     string
	Referer string
	Quality int
}

// Qualities enum de calidades
const (
	Unknown = iota
	Low
	Medium
	High
)

// INFER_TYPE valor por defecto para tipo inferido
const INFER_TYPE = -1

// newExtractorLink crea un ExtractorLink con configuración opcional
func newExtractorLink(source, name, linkURL string, setup func(*ExtractorLink)) *ExtractorLink {
	link := &ExtractorLink{
		Source: source,
		Name:   name,
		URL:    linkURL,
	}
	if setup != nil {
		setup(link)
	}
	return link
}

// generateM3u8 parsea una playlist M3U8 y retorna links
func generateM3u8(sourceName, m3u8URL, baseURL string, headers map[string]string) ([]*ExtractorLink, error) {
	client := &http.Client{}
	req, err := http.NewRequest("GET", m3u8URL, nil)
	if err != nil {
		return nil, err
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	playlist, listType, err := m3u8.DecodeFrom(strings.NewReader(string(data)), true)
	if err != nil {
		return nil, err
	}
	var links []*ExtractorLink

	if listType == m3u8.MASTER { // variantes
		master := playlist.(*m3u8.MasterPlaylist)
		for _, variant := range master.Variants {
			uri := variant.URI
			if !strings.HasPrefix(uri, "http") {
				u, _ := url.Parse(baseURL)
				uri = u.ResolveReference(&url.URL{Path: uri}).String()
			}
			quality := Unknown
			if variant.Resolution != "" {
				// formateado como WxH
				parts := strings.Split(variant.Resolution, "x")
				// parse height
				// ignoramos errores aquí
				height := 0
				fmt.Sscanf(parts[len(parts)-1], "%d", &height)
				quality = height
			}
			name := fmt.Sprintf("%s HLS", sourceName)
			if variant.Resolution != "" {
				name = fmt.Sprintf("%s %s", sourceName, variant.Resolution)
			}
			links = append(links, newExtractorLink(
				fmt.Sprintf("%s HLS", sourceName),
				name,
				uri,
				func(l *ExtractorLink) {
					l.Referer = baseURL
					l.Quality = quality
				},
			))
		}
	} else { // único
		links = append(links, newExtractorLink(
			fmt.Sprintf("%s HLS", sourceName),
			fmt.Sprintf("%s HLS", sourceName),
			m3u8URL,
			func(l *ExtractorLink) {
				l.Referer = baseURL
				l.Quality = Unknown
			},
		))
	}
	return links, nil
}

// base64Decode decodifica una cadena en base64
func base64Decode(input string) (string, error) {
	data, err := base64.StdEncoding.DecodeString(input)
	return string(data), err
}

// funciones auxiliares de decodificación
func rot13(input string) string {
	var result strings.Builder
	for _, c := range input {
		switch {
		case c >= 'A' && c <= 'Z':
			result.WriteRune('A' + (c-'A'+13)%26)
		case c >= 'a' && c <= 'z':
			result.WriteRune('a' + (c-'a'+13)%26)
		default:
			result.WriteRune(c)
		}
	}
	return result.String()
}

func replacePatterns(input string) string {
	patterns := []string{"@$", "^^", "~@", "%?", "*~", "!!", "#&"}
	out := input
	for _, p := range patterns {
		out = strings.ReplaceAll(out, p, "_")
	}
	return out
}

func removeUnderscores(input string) string {
	return strings.ReplaceAll(input, "_", "")
}

func charShift(input string, shift int) string {
	var out strings.Builder
	for _, ch := range input {
		out.WriteRune(rune(int(ch) - shift))
	}
	return out.String()
}

func reverse(input string) string {
	runes := []rune(input)
	for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
		runes[i], runes[j] = runes[j], runes[i]
	}
	return string(runes)
}

// decryptF7 aplica la lógica de decodificación original
func decryptF7(p8 string) (map[string]interface{}, error) {
	vF := rot13(p8)
	vF2 := replacePatterns(vF)
	vF3 := removeUnderscores(vF2)
	vF4, err := base64Decode(vF3)
	if err != nil {
		return nil, err
	}
	vF5 := charShift(vF4, 3)
	vF6 := reverse(vF5)
	vAtob, err := base64Decode(vF6)
	if err != nil {
		return nil, err
	}
	var result map[string]interface{}
	err = json.Unmarshal([]byte(vAtob), &result)
	return result, err
}

var redirectRegex = regexp.MustCompile(`window\.location\.href\s*=\s*'([^']+)';`)

func httpGet(rawURL, referer string) (string, error) {
	client := &http.Client{}
	req, err := http.NewRequest("GET", rawURL, nil)
	if err != nil {
		return "", err
	}
	if referer != "" {
		req.Header.Set("Referer", referer)
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	return string(body), err
}

// Extractor define la interfaz para extractores
type Extractor interface {
	Name() string
	MainURL() string
	RequiresReferer() bool
	GetURL(
		pageURL, referer string,
		subCallback func(SubtitleFile),
		linkCallback func(*ExtractorLink),
	) error
}

// VoeExtractor implementa la interfaz Extractor
type VoeExtractor struct {
	name            string
	mainURL         string
	requiresReferer bool
}

func (v *VoeExtractor) Name() string          { return v.name }
func (v *VoeExtractor) MainURL() string       { return v.mainURL }
func (v *VoeExtractor) RequiresReferer() bool { return v.requiresReferer }

func (v *VoeExtractor) GetURL(
	pageURL, referer string,
	subCallback func(SubtitleFile),
	linkCallback func(*ExtractorLink),
) error {
	html, err := httpGet(pageURL, referer)
	if err != nil {
		return err
	}
	if match := redirectRegex.FindStringSubmatch(html); len(match) > 1 {
		html, err = httpGet(match[1], referer)
		if err != nil {
			return err
		}
	}
	// parsear HTML
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return err
	}
	script := doc.Find("script[type='application/json']").First().Text()
	if script == "" {
		log.Println("encoded string not found.")
		return nil
	}
	trimmed := strings.TrimSpace(script)
	encMatch := regexp.MustCompile(`\["(.+)"\]`).FindStringSubmatch(trimmed)
	if len(encMatch) < 2 {
		log.Println("encoded string not found.")
		return nil
	}
	encoded := encMatch[1]
	decrypted, err := decryptF7(encoded)
	if err != nil {
		log.Println("Decryption error:", err)
		return nil
	}
	// extraer urls
	if m3u8URL, ok := decrypted["source"].(string); ok && m3u8URL != "" {
		links, err := generateM3u8(v.name, m3u8URL, fmt.Sprintf("%s/", v.mainURL), map[string]string{"Origin": fmt.Sprintf("%s/", v.mainURL)})
		if err == nil {
			for _, l := range links {
				linkCallback(l)
			}
		}
	}
	if mp4URL, ok := decrypted["direct_access_url"].(string); ok && mp4URL != "" {
		linkCallback(newExtractorLink(
			fmt.Sprintf("%s MP4", v.name),
			fmt.Sprintf("%s MP4", v.name),
			mp4URL,
			func(l *ExtractorLink) {
				l.Referer = pageURL
				l.Quality = Unknown
			},
		))
	}
	return nil
}

// Instancias concretas
var (
	Voe               = &VoeExtractor{name: "Voe", mainURL: "https://voe.sx", requiresReferer: true}
	Tubeless          = &VoeExtractor{name: "Tubeless", mainURL: "https://tubelessceliolymph.com", requiresReferer: true}
	Simpulumlamerop   = &VoeExtractor{name: "Simplum", mainURL: "https://simpulumlamerop.com", requiresReferer: true}
	Urochsunloath     = &VoeExtractor{name: "Uroch", mainURL: "https://urochsunloath.com", requiresReferer: true}
	NathanFromSubject = &VoeExtractor{name: "Voe", mainURL: "https://nathanfromsubject.com", requiresReferer: true}
	Yipsu             = &VoeExtractor{name: "Yipsu", mainURL: "https://yip.su", requiresReferer: true}
	MetaGnathTuggers  = &VoeExtractor{name: "Metagnath", mainURL: "https://metagnathtuggers.com", requiresReferer: true}
	Voe1              = &VoeExtractor{name: "Voe", mainURL: "https://donaldlineelse.com", requiresReferer: true}
)

// // Ejemplo de uso
// func main() {
// 	extractor := Voe
// 	err := extractor.GetURL(
// 		"https://voe.sx/e/rgupltn3w3fc",
// 		"https://voe.sx/",
// 		func(sub SubtitleFile) {
// 			fmt.Printf("Subtitle found:\n  URL: %s\n  Language: %s\n\n", sub.URL, sub.Language)
// 		},
// 		func(link *ExtractorLink) {
// 			fmt.Printf("Link found:\n  Source: %s\n  Name: %s\n  URL: %s\n  Referer: %s\n  Quality: %d\n\n",
// 				link.Source, link.Name, link.URL, link.Referer, link.Quality)
// 		},
// 	)
// 	if err != nil {
// 		log.Println("Error:", err)
// 	}
// }
