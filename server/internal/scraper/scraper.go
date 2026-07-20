package scraper

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"cpa-monitor/server/internal/model"
	"github.com/PuerkitoBio/goquery"
)

const (
	DefaultURL = "https://priceai.cc/products/chatgpt-team-business?tags=team_k12&collector=liandongShop&max=5"
	GPTPlusURL = "https://priceai.cc/products/chatgpt-plus?tags=account_unverified"
)

var (
	spaceRE       = regexp.MustCompile(`\s+`)
	priceRE       = regexp.MustCompile(`[-+]?\d+(?:[.,]\d+)?`)
	shopRE        = regexp.MustCompile(`(?i)链动小铺\s*/\s*([[:alnum:]_-]+)`)
	unavailableRE = regexp.MustCompile(`(?i)售罄|无货|缺货|下架|停售|不可购买|已售完|out\s*of\s*stock|sold\s*out|(?:^|\D)(?:库存\s*)?0(?:\D|$)`)
	relevantRE    = regexp.MustCompile(`(?i)k12|cpa|json|反代`)
)

type Client struct {
	URL        string
	HTTPClient *http.Client
	RelevantRE *regexp.Regexp
	MaxOffers  int
}

func NewClient() *Client {
	return NewClientForURLWithLimit(DefaultURL, relevantRE, 5)
}

func NewGPTPlusClient() *Client {
	return NewClientForURLWithLimit(GPTPlusURL, nil, 1000)
}

func NewClientForURL(rawURL string, relevant *regexp.Regexp) *Client {
	return NewClientForURLWithLimit(rawURL, relevant, 10)
}

func NewClientForURLWithLimit(rawURL string, relevant *regexp.Regexp, maxOffers int) *Client {
	return &Client{
		URL: rawURL, RelevantRE: relevant, MaxOffers: maxOffers,
		HTTPClient: &http.Client{Timeout: 20 * time.Second},
	}
}

func (c *Client) Fetch() ([]model.Offer, error) {
	req, err := http.NewRequest(http.MethodGet, c.URL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "CPA-Monitor/1.0 (+local subscription monitor)")
	req.Header.Set("Accept", "text/html,application/xhtml+xml")
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch offers: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("fetch offers: unexpected HTTP status %d", resp.StatusCode)
	}
	return ParseOffersWithFilterAndLimit(resp.Body, c.URL, c.RelevantRE, c.MaxOffers)
}

func ParseOffers(r io.Reader, sourceURL string) ([]model.Offer, error) {
	return ParseOffersWithFilter(r, sourceURL, relevantRE)
}

func ParseOffersWithFilter(r io.Reader, sourceURL string, relevant *regexp.Regexp) ([]model.Offer, error) {
	return ParseOffersWithFilterAndLimit(r, sourceURL, relevant, 10)
}

func ParseOffersWithFilterAndLimit(r io.Reader, sourceURL string, relevant *regexp.Regexp, maxOffers int) ([]model.Offer, error) {
	doc, err := goquery.NewDocumentFromReader(r)
	if err != nil {
		return nil, fmt.Errorf("parse offer HTML: %w", err)
	}
	base, _ := url.Parse(sourceURL)
	offers := make([]model.Offer, 0)
	doc.Find("table tbody tr").Each(func(_ int, row *goquery.Selection) {
		cells := row.Find("td")
		if cells.Length() < 5 {
			return
		}
		cell := func(i int) string {
			if i >= cells.Length() {
				return ""
			}
			return cleanText(cells.Eq(i).Text())
		}
		stockText := cell(0)
		channel := cell(1)
		title := cell(2)
		price, ok := parsePrice(cell(3))
		if !ok || (relevant != nil && !relevant.MatchString(title)) || unavailableRE.MatchString(stockText) {
			return
		}
		link, exists := row.Find("a[href]").FilterFunction(func(_ int, s *goquery.Selection) bool {
			href, _ := s.Attr("href")
			return strings.Contains(strings.ToLower(href), "/item/") || strings.Contains(cleanText(s.Text()), "购买")
		}).First().Attr("href")
		if !exists {
			link, exists = row.Find("a[href]").First().Attr("href")
		}
		if !exists || strings.TrimSpace(link) == "" {
			return
		}
		orderURL := resolveURL(base, strings.TrimSpace(link))
		if orderURL == "" {
			return
		}
		merchant, shopID := parseMerchant(channel)
		if merchantNode := cells.Eq(1).Find("span.block.truncate.font-semibold").First(); merchantNode.Length() > 0 {
			if visibleMerchant := cleanText(merchantNode.Text()); visibleMerchant != "" {
				merchant = visibleMerchant
			}
		}
		offers = append(offers, model.Offer{
			Status: "在售", Stock: stockText, Merchant: merchant, ShopID: shopID,
			Title: title, Price: price, UpdatedAt: cell(4), OrderURL: orderURL,
			ItemID: itemIDFromURL(orderURL),
		})
	})
	sort.SliceStable(offers, func(i, j int) bool { return offers[i].Price < offers[j].Price })
	if maxOffers > 0 && len(offers) > maxOffers {
		offers = offers[:maxOffers]
	}
	return offers, nil
}

func cleanText(s string) string {
	return strings.TrimSpace(spaceRE.ReplaceAllString(strings.ReplaceAll(s, " ", " "), " "))
}

func parsePrice(s string) (float64, bool) {
	match := priceRE.FindString(strings.ReplaceAll(s, ",", ""))
	if match == "" {
		return 0, false
	}
	value, err := strconv.ParseFloat(match, 64)
	return value, err == nil && value >= 0
}

func parseMerchant(channel string) (string, string) {
	match := shopRE.FindStringSubmatchIndex(channel)
	if len(match) == 4 {
		merchant := strings.TrimSpace(strings.Trim(channel[:match[0]], "-—|/ "))
		return merchant, channel[match[2]:match[3]]
	}
	return strings.TrimSpace(channel), ""
}

func resolveURL(base *url.URL, href string) string {
	u, err := url.Parse(href)
	if err != nil {
		return ""
	}
	if base != nil {
		u = base.ResolveReference(u)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return ""
	}
	return u.String()
}

func itemIDFromURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	parts := strings.Split(strings.Trim(path.Clean(u.Path), "/"), "/")
	for i := 0; i+1 < len(parts); i++ {
		if strings.EqualFold(parts[i], "item") {
			return parts[i+1]
		}
	}
	return ""
}
