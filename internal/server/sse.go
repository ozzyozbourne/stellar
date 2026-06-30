package server

import (
	"fmt"
	"net/http"
	"strings"
)

// linkElement renders the stylesheet <link> for a given content hash.
func linkElement(hash string) string {
	return fmt.Sprintf(`<link id="stellar-css" rel="stylesheet" href="/assets/css/stellar/%s">`, hash)
}

// writeLinkPatch emits a Datastar `datastar-patch-elements` SSE event that
// morphs #stellar-css to point at the new hashed stylesheet (§7 step 2).
// Datastar matches the incoming element to the live DOM by its id, so the
// browser swaps the sheet with no navigation.
func writeLinkPatch(w http.ResponseWriter, f http.Flusher, hash string) {
	fmt.Fprint(w, "event: datastar-patch-elements\n")
	fmt.Fprint(w, "data: mode outer\n")
	fmt.Fprintf(w, "data: elements %s\n", linkElement(hash))
	fmt.Fprint(w, "\n")
	f.Flush()
}

// injectHash replaces the placeholder stylesheet link in the served HTML with
// one carrying the current hash, so the first paint is already styled.
func injectHash(page, hash string) string {
	return strings.Replace(page,
		`<link id="stellar-css" rel="stylesheet">`,
		linkElement(hash), 1)
}
