package gateways

import "testing"

func TestIsSub2APIDataPackageRecognizesTypedAndMarkerlessExports(t *testing.T) {
	account := `"accounts":[{"name":"person@example.com","platform":"openai","type":"oauth","credentials":{"email":"person@example.com","access_token":"token"}}],"proxies":[]`
	for name, data := range map[string][]byte{
		"typed":      []byte(`{"type":"sub2api-data","version":1,` + account + `}`),
		"markerless": []byte(`{"exported_at":"2026-07-22T00:00:00Z",` + account + `}`),
	} {
		t.Run(name, func(t *testing.T) {
			if !IsSub2APIDataPackage(data) {
				t.Fatal("Sub2API export was not recognized")
			}
		})
	}
}

func TestIsSub2APIDataPackageDoesNotGuessOrdinaryCredentials(t *testing.T) {
	for _, data := range [][]byte{
		[]byte(`{"email":"person@example.com","access_token":"token"}`),
		[]byte(`{"accounts":[{"credentials":{"access_token":"token"}}]}`),
		[]byte(`{"type":"other","exported_at":"2026-07-22T00:00:00Z","accounts":[],"proxies":[]}`),
	} {
		if IsSub2APIDataPackage(data) {
			t.Fatalf("ordinary credential was misclassified: %s", data)
		}
	}
}
