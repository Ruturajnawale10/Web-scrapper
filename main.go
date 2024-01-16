package main

import (
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/html"
)

// Helper function to pull the href attribute from a Token
func getHref(t html.Token) (ok bool, href string) {
	// Iterate over token attributes until we find an "href"
	for _, a := range t.Attr {
		if a.Key == "href" {
			href = a.Val
			ok = true
		}
	}

	// "bare" return will return the variables (ok, href) as
	// defined in the function definition
	return
}

// go run main.go https://schier.co https://insomnia.rest

func crawl(url string, found_urls map[string]bool, lock *sync.Mutex, completionChannel chan struct{}, urlChannel chan string) {
	res, err := http.Get(url)

	defer func() {
		completionChannel <- struct{}{}
	}()

	if err != nil {
		fmt.Println("ERROR: Failed to crawl:", url)
		return
	}

	b := res.Body
	defer b.Close()

	z := html.NewTokenizer(res.Body)

	for {
		tt := z.Next()

		switch {
		case tt == html.ErrorToken:
			// End of the document, we're done
			res.Body.Close()
			completionChannel <- struct{}{}
			return
		case tt == html.StartTagToken:
			t := z.Token()
			isAnchor := t.Data == "a"
			if !isAnchor {
				continue
			}

			ok, url := getHref(t)
			if !ok {
				continue
			}

			hasProto := strings.Index(url, "http") == 0
			if hasProto {
				urlChannel <- url
			}
		}
	}
}

func main() {
	lock := sync.Mutex{}
	start_time := time.Now()
	found_urls := map[string]bool{}
	seed_urls := []string{"https://schier.co", "https://insomnia.rest"}

	completionChannel := make(chan struct{})
	urlChannel := make(chan string)
	for _, url := range seed_urls {
		go crawl(url, found_urls, &lock, completionChannel, urlChannel)
	}

	for c := 0; c < len(seed_urls); {
		select {
		case url := <-urlChannel:
			found_urls[url] = true
		case <-completionChannel:
			c += 1

		}
	}

	fmt.Printf("Unique URLs found are %v and are as follows:\n", len(found_urls))
	for url := range found_urls {
		fmt.Println(url)
	}

	end_time := time.Now()
	fmt.Printf("Total time taken: %v", end_time.Sub(start_time))
}
