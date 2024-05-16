package main

import (
	crand "crypto/rand"
	"embed"
	"encoding/hex"
	"fmt"
	"github.com/gorilla/mux"
	"io/fs"
	"log"
	"math/rand/v2"
	"net/http"
	"path"
)

const (
	cardsLen  = 272
	chartsLen = 44
)

type numberSelector struct {
	maxNum int
	used   map[int]bool
}

func newNumberSelector(maxNum int) *numberSelector {
	return &numberSelector{maxNum: maxNum, used: make(map[int]bool)}
}

func (ns *numberSelector) randomNum() int {
	for {
		r := rand.IntN(ns.maxNum + 1)
		if ns.used[r] {
			continue
		}
		ns.used[r] = true
		if len(ns.used)-1 == ns.maxNum {
			ns.reset()
		}
		return r
	}
}

func (ns *numberSelector) reset() {
	ns.used = make(map[int]bool)
}

type game struct {
	chartSelector *numberSelector
	cardSelector  *numberSelector
	player1Name   string
	player2Name   string
	player1ID     string
	player2ID     string
	conn1         *connection
	conn2         *connection
	submission1   bool
	submission2   bool
	chart         int
}

type server struct {
	games map[string]*game
}

func main() {
	s := &server{
		games: make(map[string]*game),
	}

	wsHub := newHub()
	wsHub.server = s
	go wsHub.run()

	mx := http.NewServeMux()
	r := mux.NewRouter()
	r.Methods("OPTIONS")
	r.HandleFunc("/newgame", s.handlePOSTNewGame).Methods("POST")
	r.HandleFunc("/joingame", s.handlePOSTJoinGame).Methods("POST")
	r.HandleFunc("/submitcard", s.handlePOSTSubmitCard).Methods("POST")
	r.HandleFunc("/chart", s.handleGETChart).Methods("GET")
	r.HandleFunc("/card", s.handleGETCard).Methods("GET")
	r.PathPrefix("/charts").Handler(http.HandlerFunc(s.serveStaticFile))
	r.PathPrefix("/cards").Handler(http.HandlerFunc(s.serveStaticFile))
	mx.Handle("/", r)
	mx.Handle("/ws", newWebsocketHandler(wsHub))

	if err := http.ListenAndServe(":8080", mx); err != nil {
		log.Fatal(err)
	}
}

func (s *server) handlePOSTNewGame(w http.ResponseWriter, r *http.Request) {
	name := r.URL.Query().Get("name")

	gameID := make([]byte, 8)
	crand.Read(gameID)
	userID := make([]byte, 8)
	crand.Read(userID)

	game := &game{
		chartSelector: newNumberSelector(chartsLen),
		cardSelector:  newNumberSelector(cardsLen),
		player1Name:   name,
		player1ID:     hex.EncodeToString(userID),
		submission1:   true,
		submission2:   true,
	}

	s.games[hex.EncodeToString(gameID)] = game
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	fmt.Fprint(w, fmt.Sprintf(`{"gameID": "%s", "userID": "%s"}`, hex.EncodeToString(gameID), hex.EncodeToString(userID)))
}

func (s *server) handlePOSTJoinGame(w http.ResponseWriter, r *http.Request) {
	name := r.URL.Query().Get("name")
	gameID := r.URL.Query().Get("gameID")

	if name == "" || gameID == "" {
		http.Error(w, "Missing name or gameID parameter", http.StatusBadRequest)
		return
	}

	game, ok := s.games[gameID]
	if !ok {
		http.Error(w, "Game not found", http.StatusBadRequest)
		return
	}
	userID := make([]byte, 8)
	crand.Read(userID)

	game.player2Name = name
	game.player2ID = hex.EncodeToString(userID)

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	fmt.Fprint(w, fmt.Sprintf(`{"userID": "%s", "opponent": "%s"}`, hex.EncodeToString(userID), game.player1Name))
}

func (s *server) handlePOSTSubmitCard(w http.ResponseWriter, r *http.Request) {
	gameID := r.URL.Query().Get("gameID")
	userID := r.URL.Query().Get("userID")
	card := r.URL.Query().Get("card")

	if gameID == "" || userID == "" {
		http.Error(w, "Missing gameID or userID parameter", http.StatusBadRequest)
		return
	}

	game, ok := s.games[gameID]
	if !ok {
		http.Error(w, "Game not found", http.StatusBadRequest)
		return
	}

	if userID == game.player1ID {
		if game.conn2 != nil {
			game.conn2.send <- []byte(fmt.Sprintf(`{"submit": %s}`, card))
		}
		game.submission1 = true
	}
	if userID == game.player2ID {
		if game.conn1 != nil {
			game.conn1.send <- []byte(fmt.Sprintf(`{"submit": %s}`, card))
		}
		game.submission2 = true
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	fmt.Fprint(w, "{}")
}

func (s *server) handleGETChart(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("gameID")

	game, ok := s.games[id]
	if !ok {
		http.Error(w, "Game not found", http.StatusBadRequest)
		return
	}

	if game.submission1 && game.submission2 {
		game.submission1 = false
		game.submission2 = false

		game.chart = game.chartSelector.randomNum()
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	fmt.Fprint(w, fmt.Sprintf(`{"id": %d}`, game.chart))
}

func (s *server) handleGETCard(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")

	game, ok := s.games[id]
	if !ok {
		http.Error(w, "Game not found", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	fmt.Fprint(w, fmt.Sprintf(`{"id": %d}`, game.cardSelector.randomNum()))
}

//go:embed static/*
var embeddedFiles embed.FS

func (s *server) serveStaticFile(w http.ResponseWriter, r *http.Request) {
	// Strip the "/static" prefix and clean the path
	filePath := path.Clean(r.URL.Path[len("/"):])

	// Use the embedded file system
	fileSystem, err := fs.Sub(embeddedFiles, "static")
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Create a sub file system from the specified path
	f, err := fileSystem.Open(filePath)
	if err != nil {
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}
	f.Close()

	// Serve the file from the embedded file system
	http.FileServer(http.FS(fileSystem)).ServeHTTP(w, r)
}
