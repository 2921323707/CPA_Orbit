package scraper

import (
	"strings"
	"testing"
)

func TestNewClientUsesFilteredK12FeedAndFiveOfferLimit(t *testing.T) {
	client := NewClient()
	if client.URL != "https://priceai.cc/products/chatgpt-team-business?tags=team_k12&collector=liandongShop&max=5" {
		t.Fatalf("unexpected K12 feed URL: %s", client.URL)
	}
	if client.MaxOffers != 5 {
		t.Fatalf("got max offers %d, want 5", client.MaxOffers)
	}
	if client.MaxPrice != 5 {
		t.Fatalf("got max price %.2f, want 5.00", client.MaxPrice)
	}
}

func TestNewGPTPlusClientUsesUnverifiedAccountFeed(t *testing.T) {
	client := NewGPTPlusClient()
	if client.URL != "https://priceai.cc/products/chatgpt-plus?tags=account_unverified" {
		t.Fatalf("unexpected GPT Plus feed URL: %s", client.URL)
	}
}

func TestParseOffersFiltersSortsAndExtracts(t *testing.T) {
	html := `<table><tbody>
<tr><td>在售 库存 20</td><td>甲商家 链动小铺 / SHOP_1</td><td>普通 Team</td><td>¥0.10</td><td>2026-01-01</td><td></td><td><a href="/item/nope">购买</a></td></tr>
<tr><td>售罄</td><td>乙商家</td><td>K12 CPA</td><td>¥0.20</td><td>2026-01-01</td><td></td><td><a href="/item/sold">购买</a></td></tr>
<tr><td>有货 8</td><td>奥特曼严选链动小铺 / PAXOVOVJ</td><td>1个 team k12子号 反代</td><td>¥0.46</td><td>2026-07-18 15:10</td><td></td><td><a href="https://pay.ldxp.cn/item/orahrw">前往购买</a></td></tr>
<tr><td>有货 3</td><td>丙商家</td><td>K12 子号</td><td>¥0.43</td><td>2026-07-18 15:11</td><td></td><td><a href="/item/cheap">购买</a></td></tr>
<tr><td>有货 库存 67</td><td>牟利ai</td><td>GPT BUG TEAM 账号 月限额 仅限sub2api，cpa自己解决</td><td>¥12.36</td><td>2026-07-20 22:17</td><td></td><td><a href="/item/bug-team">购买</a></td></tr>
<tr><td>有货 库存 2</td><td>丁商家</td><td>K12 子号</td><td>¥5.01</td><td>2026-07-18 15:12</td><td></td><td><a href="/item/over-price">购买</a></td></tr>
<tr><td>无货 库存 0</td><td>戊商家</td><td>K12 子号</td><td>¥0.30</td><td>2026-07-18 15:13</td><td></td><td><a href="/item/no-stock">购买</a></td></tr>
</tbody></table>`
	offers, err := ParseOffers(strings.NewReader(html), "https://priceai.cc/products/chatgpt-team-business")
	if err != nil {
		t.Fatal(err)
	}
	if len(offers) != 2 {
		t.Fatalf("got %d offers, want 2", len(offers))
	}
	if offers[0].ItemID != "cheap" || offers[0].Price != 0.43 {
		t.Fatalf("unexpected first offer: %+v", offers[0])
	}
	if offers[1].Merchant != "奥特曼严选" || offers[1].ShopID != "PAXOVOVJ" {
		t.Fatalf("merchant extraction failed: %+v", offers[1])
	}
}

func TestParseGPTPlusOffersDoesNotApplyK12TitleFilter(t *testing.T) {
	html := `<table><tbody><tr><td>有货 库存 6</td><td>Plus 商家 链动小铺 / PLUS_1</td><td>ChatGPT Plus 日抛成品号</td><td>¥3.50</td><td>2026-07-18 19:17</td><td></td><td><a href="/item/plus-1">购买</a></td></tr></tbody></table>`
	offers, err := ParseOffersWithFilter(strings.NewReader(html), "https://priceai.cc/products/chatgpt-plus", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(offers) != 1 || offers[0].Price != 3.5 || offers[0].ItemID != "plus-1" {
		t.Fatalf("unexpected GPT Plus offer: %+v", offers)
	}
}
