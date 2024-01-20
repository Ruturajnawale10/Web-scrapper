package main

import (
	"fmt"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/html"
	"web-scrapper.py/ratelimiter"
)

type Question struct {
	title string
	likes int
}

// Helper function to pull the href attribute from a Token
func getHref(t html.Token) (ok bool, href string) {
	for _, a := range t.Attr {
		if a.Key == "href" {
			href = a.Val
			ok = true
		}
	}

	return
}

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

			ok, title := getAnchorText(z)
			if ok {
				title = strings.Replace(title, "'", "", -1)
				title = strings.Replace(title, "?", "", -1)
				regex := regexp.MustCompile(`\s+`)
				title = regex.ReplaceAllString(title, " ")
				title = strings.Replace(title, " ", "-", -1)
				complete_url := "https://leetcode.com/problems/" + title
				urlChannel <- complete_url
			} else {
				urlChannel <- ""
			}

		case tt == html.TextToken:
			t := string(z.Text())
			idx := strings.Index(t, "likes")
			if idx >= 0 {
				last_idx_str := t[idx : idx+15]
				last_idx := strings.Index(last_idx_str, ",")
				first_idx := strings.Index(last_idx_str, ":") + 1
				likes := last_idx_str[first_idx:last_idx]
				fmt.Println(likes)
			}
		}
	}
}

func getAnchorText(z *html.Tokenizer) (bool, string) {
	for {
		tt := z.Next()
		switch tt {
		case html.ErrorToken:
			return false, ""
		case html.TextToken:
			return true, strings.TrimSpace(string(z.Text()))
		case html.EndTagToken:
			return false, ""
		}
	}
}

func crawlQuestion(url string, found_urls map[string]bool, lock *sync.Mutex, completionChannel2 chan struct{}, questionChannel chan Question) {
	res, err := http.Get(url)

	defer func() {
		completionChannel2 <- struct{}{}
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
			res.Body.Close()
			return

		case tt == html.TextToken:
			t := string(z.Text())
			idx := strings.Index(t, "likes")
			if idx >= 0 {
				last_idx_str := t[idx : idx+15]
				last_idx := strings.Index(last_idx_str, ",")
				first_idx := strings.Index(last_idx_str, ":") + 1
				likes := last_idx_str[first_idx:last_idx]
				likes_cnt, _ := strconv.Atoi(likes)
				questionChannel <- Question{url, likes_cnt}
			} else {
				questionChannel <- Question{}
			}
		}
	}
}

type QuestionSlice []Question

func (questions QuestionSlice) Len() int { return len(questions) }

func (question QuestionSlice) Less(i, j int) bool {
	if question[i].likes >= question[j].likes {
		return true
	}

	return false
}

func (question QuestionSlice) Swap(i, j int) { question[i], question[j] = question[j], question[i] }

func main() {
	lock := sync.Mutex{}
	start_time := time.Now()
	found_urls := map[string]bool{}
	seed_urls := []string{"https://leetcode.ca/all/problems.html"}

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

	questions_likes := []Question{}
	questionChannel := make(chan Question)
	completionChannel2 := make(chan struct{})

	question_urls := []string{}
	for url := range found_urls {
		question_urls = append(question_urls, url)
	}

	// total_questions := len(question_urls)
	total_questions := 105

	rateLimiter := ratelimiter.NewRateLimiter(20, 20)

	for i := 0; i < total_questions; i++ {
		url := question_urls[i]
		if rateLimiter.Allow() {
			fmt.Printf("Goroutine %d: Performing task\n", i)
			go crawlQuestion(url, found_urls, &lock, completionChannel2, questionChannel)
		} else {
			fmt.Println("Rate limit exceeded. Waiting...")
			time.Sleep(10 * time.Second) // Wait and retry
			i--
		}
		time.Sleep(time.Second)
	}

	for c := 0; c < total_questions; {
		select {
		case question := <-questionChannel:
			if question.title != "" {
				questions_likes = append(questions_likes, question)
			}

		case <-completionChannel2:
			c += 1
		}
	}

	fmt.Printf("Total Unique URLs found are %d:\n", len(found_urls))

	sort.Sort(QuestionSlice(questions_likes))

	printSortedQuestions(questions_likes)
	end_time := time.Now()
	fmt.Printf("Total execution time: %v", end_time.Sub(start_time))
}

func printSortedQuestions(questions []Question) {
	fmt.Println("Rank Question                   Likes")
	for i, question := range questions {
		fmt.Printf("%d. %s    %d\n", i, question.title, question.likes)
	}
}
