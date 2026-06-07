package normalize

import "testing"

func TestCanonicalURLStripsTracking(t *testing.T) {
	got := CanonicalURL("HTTPS://Example.COM/path/?utm_source=x&b=2&fbclid=abc#frag")
	want := "https://example.com/path?b=2"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}
func TestNormalizeTitle(t *testing.T) {
	got := NormalizeTitle("  Fed HOLDS rates!!! | Reuters ")
	want := "fed holds rates."
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}
func TestContentHashStability(t *testing.T) {
	a := ContentHash("The Federal Reserve announced a policy decision with several market relevant details and more text to exceed threshold for hashing stability across whitespace changes in normalized financial article content.")
	b := ContentHash("The   Federal Reserve announced a policy decision with several market relevant details and more text to exceed threshold for hashing stability across whitespace changes in normalized financial article content.")
	if a == "" || a != b {
		t.Fatalf("hashes not stable %q %q", a, b)
	}
}
func TestNearDuplicateSimilarity(t *testing.T) {
	a := "Federal Reserve holds interest rates steady and signals future cuts"
	b := "The Federal Reserve holds rates steady while signaling future cuts"
	if Similarity(a, b) <= 0 {
		t.Fatalf("expected positive similarity")
	}
}
