package main

import (
	"database/sql"
	_ "embed"
	"encoding/json"
	"log"
	"math"
	"math/rand"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
)

//go:embed places.json
var placesJSON []byte

type Place struct {
	ID          int      `json:"id"`
	Name        string   `json:"name"`
	Lat         float64  `json:"lat"`
	Lng         float64  `json:"lng"`
	PlaceType   string   `json:"placeType"`
	Description string   `json:"description"`
	Urls        []string `json:"urls"`
	Distance    *float64 `json:"distance,omitempty"`
	Rating      *float64 `json:"rating"`
	ReviewCount int      `json:"reviewCount"`
}

type FilterRequest struct {
	Lat        *float64 `json:"lat"`
	Lng        *float64 `json:"lng"`
	Radius     *float64 `json:"radius"`
	TypeFilter []string `json:"typeFilter"`
	NameFilter string   `json:"nameFilter"`
	SortBy     string   `json:"sortBy"`
}

type ErrorResponse struct {
	Error   string   `json:"error"`
	Reasons []string `json:"reasons,omitempty"`
}

var placeTypes = []string{"temple", "monument", "park", "theatre", "museum", "hotel", "restaurant", "cafe", "other"}

func loadPlaces() []Place {
	var data struct {
		Places []Place `json:"places"`
	}
	if err := json.Unmarshal(placesJSON, &data); err != nil {
		log.Fatal(err)
	}
	return data.Places
}

// distance between two points in meters (haversine)
func distance(lat1, lng1, lat2, lng2 float64) float64 {
	const r = 6371000.0
	dLat := (lat2 - lat1) * math.Pi / 180
	dLng := (lng2 - lng1) * math.Pi / 180
	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(lat1*math.Pi/180)*math.Cos(lat2*math.Pi/180)*math.Sin(dLng/2)*math.Sin(dLng/2)
	return 2 * r * math.Asin(math.Sqrt(a))
}

func filterPlaces(places []Place, f FilterRequest) []Place {
	result := []Place{}
	name := strings.ToLower(strings.TrimSpace(f.NameFilter))
	for _, p := range places {
		if f.TypeFilter != nil && !contains(f.TypeFilter, p.PlaceType) {
			continue
		}
		if name != "" && !strings.Contains(strings.ToLower(p.Name), name) {
			continue
		}
		if f.Lat != nil && f.Lng != nil && f.Radius != nil {
			d := distance(*f.Lat, *f.Lng, p.Lat, p.Lng)
			if d > *f.Radius {
				continue
			}
			p.Distance = &d
		}
		result = append(result, p)
	}
	switch f.SortBy {
	case "rating":
		sort.SliceStable(result, func(i, j int) bool { return ratingOf(result[i]) > ratingOf(result[j]) })
	case "distance":
		sort.SliceStable(result, func(i, j int) bool { return distOf(result[i]) < distOf(result[j]) })
	}
	return result
}

func ratingOf(p Place) float64 {
	if p.Rating == nil {
		return 0
	}
	return *p.Rating
}

func distOf(p Place) float64 {
	if p.Distance == nil {
		return math.MaxFloat64
	}
	return *p.Distance
}

// validate checks the filter request, returns list of problems
func validate(f FilterRequest) []string {
	var reasons []string
	if (f.Lat == nil) != (f.Lng == nil) {
		reasons = append(reasons, "lat and lng must be set together")
	}
	if f.Radius != nil && (f.Lat == nil || *f.Radius <= 0) {
		reasons = append(reasons, "radius needs lat/lng and must be > 0")
	}
	if f.SortBy != "" && f.SortBy != "rating" && f.SortBy != "distance" {
		reasons = append(reasons, "sortBy must be rating or distance")
	}
	for _, t := range f.TypeFilter {
		if !contains(placeTypes, t) {
			reasons = append(reasons, "unknown place type: "+t)
		}
	}
	return reasons
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

type server struct {
	db       *sql.DB
	places   []Place
	delay    time.Duration
	failRate float64
}

func (s *server) routes(staticDir string) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /place", s.getPlaces)
	mux.HandleFunc("GET /place/{id}", s.getPlace)
	mux.HandleFunc("POST /filtered_places", s.getFiltered)
	mux.HandleFunc("GET /place/{id}/reviews", s.getReviews)
	mux.HandleFunc("POST /place/{id}/reviews", s.postReview)
	if staticDir != "" {
		mux.Handle("GET /", http.FileServer(http.Dir(staticDir)))
	}
	return logRequests(mux)
}

// chaos adds a delay and random server errors, to see loaders and retries in the app
func (s *server) chaos(w http.ResponseWriter) bool {
	time.Sleep(s.delay)
	if s.failRate > 0 && rand.Float64() < s.failRate {
		writeJSON(w, http.StatusServiceUnavailable, ErrorResponse{Error: "service unavailable"})
		return true
	}
	return false
}

func (s *server) getPlaces(w http.ResponseWriter, r *http.Request) {
	if s.chaos(w) {
		return
	}
	places, err := s.placesWithRatings()
	if err != nil {
		internalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, places)
}

// placesWithRatings returns a copy of places with rating and review count filled
func (s *server) placesWithRatings() ([]Place, error) {
	info, err := ratings(s.db)
	if err != nil {
		return nil, err
	}
	places := make([]Place, len(s.places))
	for i, p := range s.places {
		if r, ok := info[p.ID]; ok {
			avg := math.Round(r.avg*10) / 10
			p.Rating = &avg
			p.ReviewCount = r.count
		}
		places[i] = p
	}
	return places, nil
}

// findPlace parses {id} from the path and writes 400/404 if there is no such place
func (s *server) findPlace(w http.ResponseWriter, r *http.Request) (Place, bool) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "bad id"})
		return Place{}, false
	}
	places, err := s.placesWithRatings()
	if err != nil {
		internalError(w, err)
		return Place{}, false
	}
	for _, p := range places {
		if p.ID == id {
			return p, true
		}
	}
	writeJSON(w, http.StatusNotFound, ErrorResponse{Error: "place not found"})
	return Place{}, false
}

func internalError(w http.ResponseWriter, err error) {
	log.Println("error:", err)
	writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "internal error"})
}

func (s *server) getPlace(w http.ResponseWriter, r *http.Request) {
	if s.chaos(w) {
		return
	}
	if p, ok := s.findPlace(w, r); ok {
		writeJSON(w, http.StatusOK, p)
	}
}

func (s *server) getReviews(w http.ResponseWriter, r *http.Request) {
	if s.chaos(w) {
		return
	}
	p, ok := s.findPlace(w, r)
	if !ok {
		return
	}
	reviews, err := getReviews(s.db, p.ID)
	if err != nil {
		internalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, reviews)
}

func (s *server) postReview(w http.ResponseWriter, r *http.Request) {
	if s.chaos(w) {
		return
	}
	p, ok := s.findPlace(w, r)
	if !ok {
		return
	}
	var req ReviewRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "bad json", Reasons: []string{err.Error()}})
		return
	}
	if reasons := validateReview(req); len(reasons) > 0 {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "bad review", Reasons: reasons})
		return
	}
	review, err := addReview(s.db, p.ID, req)
	if err != nil {
		internalError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, review)
}

func (s *server) getFiltered(w http.ResponseWriter, r *http.Request) {
	if s.chaos(w) {
		return
	}
	var f FilterRequest
	if err := json.NewDecoder(r.Body).Decode(&f); err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "bad json", Reasons: []string{err.Error()}})
		return
	}
	if reasons := validate(f); len(reasons) > 0 {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "bad filter", Reasons: reasons})
		return
	}
	places, err := s.placesWithRatings()
	if err != nil {
		internalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, filterPlaces(places, f))
}

func logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/assets") {
			log.Println(r.Method, r.URL.Path)
		}
		next.ServeHTTP(w, r)
	})
}

func envFloat(name string, def float64) float64 {
	v, err := strconv.ParseFloat(os.Getenv(name), 64)
	if err != nil {
		return def
	}
	return v
}

func main() {
	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "reviews.db"
	}
	db, err := openDB(dbPath)
	if err != nil {
		log.Fatal(err)
	}
	s := &server{
		db:       db,
		places:   loadPlaces(),
		delay:    time.Duration(envFloat("DELAY_MS", 0)) * time.Millisecond,
		failRate: envFloat("FAIL_RATE", 0),
	}
	addr := ":" + os.Getenv("PORT")
	if addr == ":" {
		addr = ":8888"
	}
	log.Printf("listening on %s, %d places", addr, len(s.places))
	log.Fatal(http.ListenAndServe(addr, s.routes(os.Getenv("STATIC_DIR"))))
}
