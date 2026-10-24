package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func newServer(t *testing.T) *server {
	db, err := openDB(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return &server{db: db, places: loadPlaces()}
}

func newTestServer(t *testing.T) http.Handler {
	return newServer(t).routes("")
}

func do(h http.Handler, method, url, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, url, strings.NewReader(body))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestDistance(t *testing.T) {
	// Moscow - Saint Petersburg is about 634 km
	d := distance(55.7558, 37.6173, 59.9343, 30.3351)
	if d < 620000 || d > 650000 {
		t.Fatalf("unexpected distance %f", d)
	}
	if distance(10, 10, 10, 10) != 0 {
		t.Fatal("distance to itself must be 0")
	}
}

func TestGetPlaces(t *testing.T) {
	rec := do(newTestServer(t), "GET", "/place", "")
	if rec.Code != 200 {
		t.Fatalf("code %d", rec.Code)
	}
	var places []Place
	json.Unmarshal(rec.Body.Bytes(), &places)
	if len(places) != 70 {
		t.Fatalf("expected 70 places, got %d", len(places))
	}
}

func TestGetPlaceDetails(t *testing.T) {
	h := newTestServer(t)
	rec := do(h, "GET", "/place/1", "")
	var p Place
	json.Unmarshal(rec.Body.Bytes(), &p)
	if rec.Code != 200 || p.Name != "Государственный Эрмитаж" {
		t.Fatalf("code %d, name %q", rec.Code, p.Name)
	}
	if do(h, "GET", "/place/999", "").Code != 404 {
		t.Fatal("expected 404")
	}
	if do(h, "GET", "/place/abc", "").Code != 400 {
		t.Fatal("expected 400")
	}
}

func TestFilteredPlaces(t *testing.T) {
	h := newTestServer(t)
	body := `{"lat":55.754194,"lng":37.620139,"radius":3000,"typeFilter":["museum","park"]}`
	rec := do(h, "POST", "/filtered_places", body)
	if rec.Code != 200 {
		t.Fatalf("code %d: %s", rec.Code, rec.Body)
	}
	var places []Place
	json.Unmarshal(rec.Body.Bytes(), &places)
	if len(places) == 0 {
		t.Fatal("expected some places")
	}
	for _, p := range places {
		if p.PlaceType != "museum" && p.PlaceType != "park" {
			t.Fatalf("wrong type %s", p.PlaceType)
		}
		if p.Distance == nil || *p.Distance > 3000 {
			t.Fatalf("place %s is out of radius", p.Name)
		}
	}
}

func TestFilteredByName(t *testing.T) {
	rec := do(newTestServer(t), "POST", "/filtered_places", `{"nameFilter":"эрмитаж"}`)
	var places []Place
	json.Unmarshal(rec.Body.Bytes(), &places)
	if len(places) != 2 {
		t.Fatalf("expected 2 places, got %d", len(places))
	}
}

func TestFilteredEmptyTypes(t *testing.T) {
	rec := do(newTestServer(t), "POST", "/filtered_places", `{"typeFilter":[]}`)
	if strings.TrimSpace(rec.Body.String()) != "[]" {
		t.Fatalf("expected empty list, got %s", rec.Body)
	}
}

func TestFilteredBadRequest(t *testing.T) {
	h := newTestServer(t)
	if do(h, "POST", "/filtered_places", `{bad`).Code != 400 {
		t.Fatal("expected 400 for bad json")
	}
	rec := do(h, "POST", "/filtered_places", `{"typeFilter":["zoo"]}`)
	var e ErrorResponse
	json.Unmarshal(rec.Body.Bytes(), &e)
	if rec.Code != 400 || len(e.Reasons) != 1 {
		t.Fatalf("code %d, reasons %v", rec.Code, e.Reasons)
	}
}

func TestChaos(t *testing.T) {
	s := newServer(t)
	s.failRate = 1
	if do(s.routes(""), "GET", "/place", "").Code != 503 {
		t.Fatal("expected 503 with failRate=1")
	}
}

func TestReviews(t *testing.T) {
	h := newTestServer(t)
	rec := do(h, "POST", "/place/1/reviews", `{"author":"Женя","rating":5,"text":"Отлично"}`)
	if rec.Code != 201 {
		t.Fatalf("code %d: %s", rec.Code, rec.Body)
	}
	do(h, "POST", "/place/1/reviews", `{"author":"Аня","rating":4,"text":""}`)

	var reviews []Review
	json.Unmarshal(do(h, "GET", "/place/1/reviews", "").Body.Bytes(), &reviews)
	if len(reviews) != 2 || reviews[0].Author != "Аня" {
		t.Fatalf("unexpected reviews %+v", reviews)
	}

	var p Place
	json.Unmarshal(do(h, "GET", "/place/1", "").Body.Bytes(), &p)
	if p.Rating == nil || *p.Rating != 4.5 || p.ReviewCount != 2 {
		t.Fatalf("unexpected rating %v, count %d", p.Rating, p.ReviewCount)
	}
}

func TestReviewValidation(t *testing.T) {
	h := newTestServer(t)
	rec := do(h, "POST", "/place/1/reviews", `{"author":"","rating":7}`)
	var e ErrorResponse
	json.Unmarshal(rec.Body.Bytes(), &e)
	if rec.Code != 400 || len(e.Reasons) != 2 {
		t.Fatalf("code %d, reasons %v", rec.Code, e.Reasons)
	}
	if do(h, "POST", "/place/999/reviews", `{"author":"a","rating":3}`).Code != 404 {
		t.Fatal("expected 404 for unknown place")
	}
}

func TestSortByRating(t *testing.T) {
	h := newTestServer(t)
	do(h, "POST", "/place/5/reviews", `{"author":"a","rating":5}`)
	do(h, "POST", "/place/2/reviews", `{"author":"a","rating":3}`)
	var places []Place
	json.Unmarshal(do(h, "POST", "/filtered_places", `{"sortBy":"rating"}`).Body.Bytes(), &places)
	if places[0].ID != 5 || places[1].ID != 2 {
		t.Fatalf("wrong order: %d, %d", places[0].ID, places[1].ID)
	}
}
