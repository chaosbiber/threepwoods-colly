package main

import (
	"flag"
	"fmt"
	"log"
	"net/url"
	"os"
	"regexp"
	"strings"
	"sync"

	"golang.org/x/exp/slices"

	"github.com/gocolly/colly/v2"
)

type ScanResult struct {
	visits                   uint32
	googleAnalyticsScriptSrc bool
	googleAnalyticsScript    bool
	googleAnalyticsIFrame    bool
	googleFontsLink          bool
	googleFontsCss           []string
	googleFontsStyle         []string
	googleFontsScript        bool
	otherLinks               []string
	otherScripts             []string
	otherIFrames             []string
	otherCss                 []string
	otherPreconnect          []string
	otherStyle               []string
	dnsPrefetch              bool
	mu                       sync.Mutex
}

var (
	verbose *bool
	depth   *int
)

func printResult(scanResult *ScanResult) {
	colorReset := "\033[0m"
	colorRed := "\033[31m"
	colorYellow := "\033[33m"
	//colorGreen := "\033[32m"
	//colorBlue := "\033[34m"
	//colorPurple := "\033[35m"
	//colorCyan := "\033[36m"
	//colorWhite := "\033[37m"

	fmt.Printf(colorRed)
	if scanResult.googleAnalyticsScriptSrc {
		fmt.Println("Website uses Google Analytics via <script src>")
	}
	if scanResult.googleAnalyticsIFrame {
		fmt.Println("Website uses Google Analytics via <iframe>")
	}
	if scanResult.googleFontsLink {
		fmt.Println("Website uses Google Fonts via <link>")
	}
	if len(scanResult.googleFontsCss) > 0 {
		fmt.Print("Website uses Google Fonts in css file @import: ")
		fmt.Printf(colorReset)
		fmt.Println(strings.Join(scanResult.googleFontsCss[:], ", "))
		fmt.Printf(colorRed)
	}
	if len(scanResult.googleFontsStyle) > 0 {
		fmt.Print("Website uses Google Fonts in <style> @import: ")
		fmt.Printf(colorReset)
		fmt.Println(strings.Join(scanResult.googleFontsStyle[:], ", "))
		fmt.Printf(colorRed)
	}
	fmt.Printf(colorReset)

	fmt.Printf(colorYellow)
	if scanResult.googleAnalyticsScript {
		fmt.Print("Found Google Analytics URL in <script>")
		fmt.Printf(colorReset)
		fmt.Println(" (this doesn't imply that it gets executed)")
		fmt.Printf(colorYellow)
	}
	if scanResult.googleFontsScript {
		fmt.Print("Found Google Fonts URL in <script>")
		fmt.Printf(colorReset)
		fmt.Println(" (this doesn't imply that it gets executed)")
		fmt.Printf(colorYellow)
	}
	if len(scanResult.otherLinks) > 0 {
		fmt.Print("Found 3rd Party <link> elements: ")
		fmt.Printf(colorReset)
		fmt.Println(strings.Join(scanResult.otherLinks[:], ", "))
		fmt.Printf(colorYellow)
	}
	if len(scanResult.otherScripts) > 0 {
		fmt.Print("Found 3rd Party <script> elements: ")
		fmt.Printf(colorReset)
		fmt.Println(strings.Join(scanResult.otherScripts[:], ", "))
		fmt.Printf(colorYellow)
	}
	if len(scanResult.otherIFrames) > 0 {
		fmt.Print("Found 3rd Party <iframe> elements: ")
		fmt.Printf(colorReset)
		fmt.Println(strings.Join(scanResult.otherIFrames[:], ", "))
		fmt.Printf(colorYellow)
	}
	if len(scanResult.otherCss) > 0 {
		fmt.Print("Found 3rd Party @import in css: ")
		fmt.Printf(colorReset)
		fmt.Println(strings.Join(scanResult.otherCss[:], ", "))
		fmt.Printf(colorYellow)
	}
	if len(scanResult.otherPreconnect) > 0 {
		fmt.Print("Found 3rd Party <link rel='preconnect'> elements: ")
		fmt.Printf(colorReset)
		fmt.Println(strings.Join(scanResult.otherPreconnect[:], ", "))
		fmt.Printf(colorYellow)
	}
	if len(scanResult.otherStyle) > 0 {
		fmt.Print("Found 3rd Party @import|s in <style> element: ")
		fmt.Printf(colorReset)
		fmt.Println(strings.Join(scanResult.otherStyle[:], ", "))
		fmt.Printf(colorYellow)
	}
	fmt.Printf(colorReset)

	if scanResult.dnsPrefetch {
		fmt.Println("Found <link rel='dns-prefetch'> elements")
	}
}

func printProgress(count uint32) {
	removeLine := "\033[2K"

	fmt.Printf(removeLine)
	fmt.Printf("\r")
	fmt.Printf("%d pages visited", count)
}

func isSameDomain(url, baseUrl, domain string) bool {
	// regex should match all possible relative paths
	localLink, err := regexp.MatchString("^(/?[a-zA-Z0-9-_.]+)*([#?].*)?$", url)
	if err != nil {
		log.Fatal("error using regex in `isSameDomain`")
	}
	if localLink {
		return true
	}
	if strings.HasPrefix(url, "//"+domain) {
		return true
	}
	if strings.HasPrefix(url, baseUrl) {
		return true
	}
	if url == "about:blank" {
		return true
	}
	return false
}

func checkUrl(urlString string) {
	// match multiple @import styles
	cssRegexp, err := regexp.Compile(`@import\W?(url)?\(?['"]?([^\)"']*)['"]?\)?`)
	if err != nil {
		log.Fatal("error compiling regexp in checkUrl()")
	}
	u, err := url.Parse(urlString)
	if err != nil {
		log.Fatal("error compiling regexp in checkUrl()")
	}
	domain := u.Hostname()

	protocol := strings.Split(urlString, ":")[0]
	if protocol != "https" && protocol != "http" {
		protocol = "https" // default if none defined
	}

	baseUrl := protocol + "://" + domain
	if u.Port() != "" {
		baseUrl += ":" + u.Port()
	}

	fmt.Println("crawling", urlString)

	var scanResult ScanResult

	c := colly.NewCollector(
		colly.AllowedDomains(domain),
		colly.MaxDepth(*depth),
		colly.Async(true),
	)

	// Find and visit all links
	c.OnHTML("a[href]", func(e *colly.HTMLElement) {
		e.Request.Visit(e.Attr("href"))
	})

	c.OnRequest(func(r *colly.Request) {
		scanResult.mu.Lock()
		defer scanResult.mu.Unlock()
		scanResult.visits += 1
		if !*verbose {
			printProgress(scanResult.visits)
		} else {
			fmt.Println("VISITING:", r.URL)
		}
	})

	c.OnHTML("link[href]", func(e *colly.HTMLElement) {
		scanResult.mu.Lock()
		defer scanResult.mu.Unlock()
		href := e.Attr("href")
		e.Request.Visit(href)
		thirdParty := !isSameDomain(href, baseUrl, domain)

		if e.Attr("rel") == "dns-prefetch" {
			scanResult.dnsPrefetch = true
			if *verbose {
				fmt.Printf("DNS-PREFETCH on %s: %s, rel: %s, id: %s\n", e.Request.URL, e.Attr("href"), e.Attr("rel"), e.Attr("id"))
			}
			return
		}

		if e.Attr("rel") == "preconnect" && thirdParty {
			if !slices.Contains(scanResult.otherPreconnect, href) {
				scanResult.otherPreconnect = append(scanResult.otherPreconnect, href)
			}
			if *verbose {
				fmt.Printf("LINK / PRECONNECT on %s: %s, rel: %s, id: %s\n", e.Request.URL, e.Attr("href"), e.Attr("rel"), e.Attr("id"))
			}
			return
		}

		if strings.Contains(href, "fonts.googleapis.com") || strings.Contains(href, "fonts.gstatic.com") {
			scanResult.googleFontsLink = true
			if *verbose {
				fmt.Printf("LINK / GOOGLEFONT on %s: %s, rel: %s, id: %s\n", e.Request.URL, e.Attr("href"), e.Attr("rel"), e.Attr("id"))
			}
			return
		}

		if thirdParty {
			if !slices.Contains(scanResult.otherLinks, href) {
				scanResult.otherLinks = append(scanResult.otherLinks, href)
			}
			if *verbose {
				fmt.Printf("3RD PARTY LINK on %s: %s, rel: %s, id: %s\n", e.Request.URL, e.Attr("href"), e.Attr("rel"), e.Attr("id"))
			}
			return
		}
	})

	c.OnHTML("script", func(e *colly.HTMLElement) {
		scanResult.mu.Lock()
		defer scanResult.mu.Unlock()
		src := e.Attr("src")

		if src != "" {
			thirdParty := !isSameDomain(src, baseUrl, domain)
			if strings.Contains(src, "googletagmanager.com") {
				scanResult.googleAnalyticsScriptSrc = true
				if *verbose {
					fmt.Printf("GOOGLE ANALYTICS <script> sourced on %s: %s\n", e.Request.URL, src)
				}
				return
			}
			if thirdParty {
				if !slices.Contains(scanResult.otherScripts, src) {
					scanResult.otherScripts = append(scanResult.otherScripts, src)
				}
				if *verbose {
					fmt.Printf("3RD PARTY <script> sourced on %s: %s\n", e.Request.URL, src)
				}
				return
			}
		}
		if strings.Contains(e.Text, "googletagmanager.com") {
			scanResult.googleAnalyticsScript = true
			if *verbose {
				fmt.Printf("GOOGLE ANALYTICS URL found in <script> on %s (unknown if that code executed)\n", e.Request.URL)
			}
			return
		}
		if strings.Contains(e.Text, "fonts.googleapis.com") {
			scanResult.googleFontsScript = true
			if *verbose {
				fmt.Printf("GOOGLE FONTS URL found in <script> on %s (unknown if that code is executed)\n", e.Request.URL)
			}
		}
	})

	c.OnHTML("iframe[src]", func(e *colly.HTMLElement) {
		scanResult.mu.Lock()
		defer scanResult.mu.Unlock()
		src := e.Attr("src")

		if src != "" {
			thirdParty := !isSameDomain(src, baseUrl, domain)
			if strings.Contains(src, "googletagmanager.com") {
				scanResult.googleAnalyticsIFrame = true
				if *verbose {
					fmt.Printf("GOOGLE ANALYTICS <iframe> sourced on %s: %s\n", e.Request.URL, src)
				}
				return
			}
			if thirdParty {
				if !slices.Contains(scanResult.otherIFrames, src) {
					scanResult.otherIFrames = append(scanResult.otherIFrames, src)
				}
				if *verbose {
					fmt.Printf("3RD PARTY <iframe> sourced on %s: %s\n", e.Request.URL, src)
				}
				return
			}
		}
	})

	c.OnHTML("style", func(e *colly.HTMLElement) {
		if e.Text != "" {
			if cssRegexp.MatchString(e.Text) {
				result := cssRegexp.FindAllStringSubmatch(e.Text, -1)
				for _, m := range result {
					sm := m[2]
					if strings.Contains(sm, "googleapis.com") {
						if !slices.Contains(scanResult.googleFontsStyle, sm) {
							scanResult.googleFontsStyle = append(scanResult.googleFontsStyle, sm)
						}
						if *verbose {
							fmt.Printf("STYLE / GOOGLEFONT @import in %s: %s\n", e.Request.URL, sm)
						}
						continue
					}
					thirdParty := !isSameDomain(sm, baseUrl, domain)
					if thirdParty {
						if !slices.Contains(scanResult.otherStyle, sm) {
							scanResult.otherStyle = append(scanResult.otherStyle, sm)
						}
						if *verbose {
							fmt.Printf("3RD PARTY @import in <style> %s: %s\n", e.Request.URL, sm)
						}
						continue
					}
				}
			}
		}
	})

	c.OnResponse(func(r *colly.Response) {
		if strings.HasSuffix(r.Request.URL.Path, "css") {

			body := string(r.Body)
			if cssRegexp.MatchString(body) {
				result := cssRegexp.FindAllStringSubmatch(body, -1)
				for _, m := range result {
					sm := m[2]
					if strings.Contains(sm, "googleapis.com") {
						if !slices.Contains(scanResult.googleFontsCss, sm) {
							scanResult.googleFontsCss = append(scanResult.googleFontsCss, sm)
						}
						if *verbose {
							fmt.Printf("CSS / GOOGLEFONT @import in %s: %s\n", urlString+r.Request.URL.Path, sm)
						}
						continue
					}
					thirdParty := !isSameDomain(sm, baseUrl, domain)
					if thirdParty {
						if !slices.Contains(scanResult.otherCss, sm) {
							scanResult.otherCss = append(scanResult.otherCss, sm)
						}
						if *verbose {
							fmt.Printf("3RD PARTY @import in css file %s: %s\n", urlString+r.Request.URL.Path, sm)
						}
						continue
					}
				}
			}
		}
	})

	c.Visit(urlString)
	c.Wait()
	fmt.Println()
	printResult(&scanResult)
}

func main() {
	depth = flag.Int("d", 3, "max depth for page visits when following links")
	verbose = flag.Bool("v", false, "verbose output")
	flag.Parse()
	values := flag.Args()
	if len(values) == 0 {
		fmt.Println("Usage: threepwoods-colly [-d 3] [-v] http://website.com")
		flag.PrintDefaults()
		os.Exit(1)
	}
	checkUrl(values[0])
}
