package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Response struct {
	Text         string `json:"text"`
	ResponseType string `json:"response_type"`
}

func (r Response) Bytes() []byte {
	buf, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		log.Print(err)
		return []byte("an error occured: " + err.Error())
	}
	return buf
}

func parseCommand(m string) string {
	return strings.Split(m, " ")[0]
}

func parseStart(t string) (string, int, string, error) {
	s := strings.SplitN(t, " ", 3)

	if len(s) < 2 {
		return "", 0, "", fmt.Errorf("quantity must be set: /cg start 100 chicken nuggets")
	}

	qty, err := strconv.Atoi(s[1])
	if err != nil {
		return "", 0, "", fmt.Errorf("quantity could not be parsed: %w", err)
	}

	var unit string
	if len(s) > 2 {
		unit = s[2]
	}

	return s[0], qty, unit, nil
}

var currWager *Wager = nil

type Wager struct {
	Qty       int
	Unit      string
	Rolls     []*Roll
	CreatedAt time.Time
	Callback  string
}

func (w *Wager) AddRoll(userID string) (*Roll, error) {
	// Verify no re-roll
	for _, roll := range w.Rolls {
		if roll.UserID == userID {
			return nil, fmt.Errorf("cannot reroll")
		}
	}

	r := &Roll{
		UserID: userID,
		Value:  rand.Intn(w.Qty),
	}
	w.Rolls = append(w.Rolls, r)
	return r, nil
}

func (w *Wager) Finalize() (*Response, error) {
	var (
		winnerUserID, loserUserID string
		winnerRoll, loserRoll     int
	)

	// calculate winner, loser, amount
	for _, r := range w.Rolls {
		log.Printf("%+v", r)
		if winnerUserID == "" {
			winnerUserID = r.UserID
			winnerRoll = r.Value
		}

		if loserUserID == "" {
			loserUserID = r.UserID
			loserRoll = r.Value
		}

		if r.Value > loserRoll {
			loserUserID = r.UserID
			loserRoll = r.Value
		}

		if r.Value < winnerRoll {
			winnerUserID = r.UserID
			winnerRoll = r.Value
		}
	}

	delta := loserRoll - winnerRoll

	resp := Response{
		Text:         fmt.Sprintf("<@%s> owes <@%s> %d %s", loserUserID, winnerUserID, delta, w.Unit),
		ResponseType: "in_channel",
	}.Bytes()

	log.Print(string(resp))

	_, err := http.DefaultClient.Post(
		w.Callback,
		"application/json",
		bytes.NewReader(resp))

	return &Response{}, err
}

type Roll struct {
	UserID string
	Value  int
}

var defaultFinalizationPeriod = 30 * time.Second

func start(f url.Values) (*Response, error) {
	m.Lock()
	defer m.Unlock()

	if currWager != nil {
		return OtherActiveWager, nil
	}

	_, qty, unit, err := parseStart(f.Get("text"))
	if err != nil {
		return nil, err
	}

	if qty <= 0 {
		return NonZeroResponse, nil
	}

	currWager = &Wager{
		Qty:      qty,
		Unit:     unit,
		Callback: f.Get("response_url"),
	}

	go func() {
		<-time.After(defaultFinalizationPeriod)
		_, err := currWager.Finalize()
		if err != nil {
			log.Printf("failed to finalize: %s", err)
		}
		currWager = nil
	}()

	return &Response{
		Text: fmt.Sprintf("<@%s> started a game: %d %s\nUse `/cg roll` to join",
			f.Get("user_id"), currWager.Qty, currWager.Unit),
		ResponseType: "in_channel",
	}, nil
}

var NonZeroResponse = &Response{
	Text: "wager quantity must be greater than zero",
}
var NoActiveWager = &Response{
	Text: "there is not an active wager. use `/cg start` to create one",
}
var OtherActiveWager = &Response{
	Text: "there's another active wager. it must finish before another can be created",
}
var m sync.Mutex

func roll(f url.Values) (*Response, error) {
	m.Lock()
	defer m.Unlock()

	if currWager == nil {
		return NoActiveWager, nil
	}

	userID := f.Get("user_id")
	roll, err := currWager.AddRoll(userID)
	if err != nil {
		return &Response{Text: err.Error()}, nil
	}

	return &Response{
		Text:         fmt.Sprintf("<@%s> rolled %d", userID, roll.Value),
		ResponseType: "in_channel",
	}, nil
}

func help(f url.Values) (*Response, error) {
	return nil, nil
}

func exec(f url.Values) (*Response, error) {
	switch parseCommand(f.Get("text")) {
	case "start":
		return start(f)
	case "roll":
		return roll(f)
	default:
		return help(f)
	}
}

func handler(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()

	resp, err := exec(r.Form)
	if err != nil {
		log.Print(err)
		http.Error(w, err.Error(), 500)
		return
	}

	_, err = http.DefaultClient.Post(
		r.Form.Get("response_url"),
		"application/json",
		bytes.NewReader(resp.Bytes()))
	if err != nil {
		log.Printf("failed to send response: %s", err)
		http.Error(w, err.Error(), 500)
		return
	}

	log.Print(resp.Bytes())
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	addr := fmt.Sprintf("0.0.0.0:%s", port)

	mux := http.NewServeMux()
	mux.HandleFunc("/", handler)

	log.Printf("Listening on http://%s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatal(err)
	}
}
