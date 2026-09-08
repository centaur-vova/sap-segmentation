package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
)

type Item struct {
	AddressSapID string `json:"address_sap_id"`
	AdrSegment   string `json:"adr_segment"`
	SegmentID    int64  `json:"segment_id"`
}

type Response struct {
	Items []Item `json:"items"`
}

const (
	validUserAgent = "spacecount-test"
	validAuth      = "4Dfddf5:jKlljHGH"
	totalRecords   = 120
)

func main() {
	http.HandleFunc("/ords/bsm/segmentation/get_segmentation", handleSegmentation)
	http.HandleFunc("/health", handleHealth)

	log.Println("Mock ERP server running on :8090")
	log.Printf("Total records: %d", totalRecords)
	log.Fatal(http.ListenAndServe(":8090", nil))
}

func handleSegmentation(w http.ResponseWriter, r *http.Request) {
	log.Printf("Request: %s %s", r.Method, r.URL.String())

	if !checkAuth(r) {
		log.Println("Unauthorized")
		w.Header().Set("WWW-Authenticate", `Basic realm="Restricted"`)
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	if r.Header.Get("User-Agent") != validUserAgent {
		log.Printf("Bad User-Agent: %s", r.Header.Get("User-Agent"))
		http.Error(w, "Bad User-Agent", http.StatusBadRequest)
		return
	}

	limit, _ := strconv.Atoi(r.URL.Query().Get("p_limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("p_offset"))

	if limit <= 0 || offset < 1 {
		http.Error(w, "Invalid parameters", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	if offset > totalRecords {
		sendJSON(w, Response{Items: []Item{}})
		return
	}

	count := limit
	if offset+count > totalRecords {
		count = totalRecords - offset + 1
	}

	items := make([]Item, 0, count)
	for i := 0; i < count; i++ {
		idx := offset + i
		items = append(items, Item{
			AddressSapID: fmt.Sprintf("SAP-ID-%04d", idx),
			AdrSegment:   fmt.Sprintf("SEG-%d", (idx%3)+1),
			SegmentID:    int64(1000 + idx),
		})
	}

	log.Printf("Response: %d items", len(items))
	sendJSON(w, Response{Items: items})
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	sendJSON(w, map[string]string{"status": "ok"})
}

func checkAuth(r *http.Request) bool {
	auth := r.Header.Get("Authorization")
	if !strings.HasPrefix(auth, "Basic ") {
		return false
	}

	decoded, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(auth, "Basic "))
	if err != nil {
		return false
	}

	return string(decoded) == validAuth
}

func sendJSON(w http.ResponseWriter, data any) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Printf("Error encoding JSON response: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}
