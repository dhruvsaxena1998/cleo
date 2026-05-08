package ids

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
)

var nameAdjectives = []string{
	"admiring", "brave", "calm", "clever", "curious", "daring", "eager", "focused",
	"gentle", "happy", "jolly", "kind", "lucid", "mighty", "nimble", "patient",
	"quiet", "sharp", "steady", "trusting", "vivid", "wise", "zesty",
}

var nameNouns = []string{
	"ada", "archimedes", "curie", "darwin", "edison", "einstein", "faraday", "franklin",
	"galileo", "hopper", "hypatia", "lovelace", "newton", "noether", "pasteur", "ramanujan",
	"tesla", "turing", "yonath",
}

func RandomName(existing map[string]bool) string {
	total := len(nameAdjectives) * len(nameNouns)
	if total == 0 {
		return DedupeSlug("agent", existing)
	}
	start := randomIndex(total)
	for i := 0; i < total; i++ {
		name := nameAt((start + i) % total)
		if !existing[name] {
			return name
		}
	}
	return DedupeSlug(nameAt(start), existing)
}

func nameAt(i int) string {
	adj := nameAdjectives[i%len(nameAdjectives)]
	noun := nameNouns[(i/len(nameAdjectives))%len(nameNouns)]
	return fmt.Sprintf("%s-%s", adj, noun)
}

func randomIndex(max int) int {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return 0
	}
	return int(binary.BigEndian.Uint64(b[:]) % uint64(max))
}
