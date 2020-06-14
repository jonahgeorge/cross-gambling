package main

import (
	"net/url"
	"testing"
	"time"
)

func TestParseStart(t *testing.T) {
	str := "start 100 chicken nuggets"

	cmd, qty, unit, err := parseStart(str)
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("cmd=%s qty=%d unit=%s", cmd, qty, unit)
}

func TestGolden(t *testing.T) {
	defaultFinalizationPeriod = 1 * time.Second

	res, err := start(url.Values{
		"text":    []string{"start 100 chicken nuggets"},
		"user_id": []string{"jonah"},
	})
	t.Logf("%+v %+v", res, err)

	res, err = roll(url.Values{"user_id": []string{"ty"}})
	t.Logf("%+v %+v", res, err)

	res, err = roll(url.Values{"user_id": []string{"john"}})
	t.Logf("%+v %+v", res, err)

	time.Sleep(5 * time.Second)
}
