package scrapper

import (
	"io"
	"net/http"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

func ExtractWebsiteContent(url string) (string, error) {
	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err

	}

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(string(body)))
	if err != nil {
		return "", err
	}
	var text string
	doc.Find("p").Each(func(i int, s *goquery.Selection) {
		text += s.Text() + " "
	})
	if len(text) > 2000 {
		text = text[:1000]
	}
	return text, nil
}
