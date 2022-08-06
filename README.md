Threepwoods Colly / GDPR Scanner
===
Scans for 3rd party connections on websites, focused on Google Fonts and Analytics, which may violate the GDPR. For crawling it utilizes the [colly](http://go-colly.org/) [library](https://github.com/gocolly/colly) for golang.

The combination of the GDPR and the Schrems II ruling at the European Court of Justice may prohibit the transfer of user IP addresses to US companies. According to the Munich Regional Court, the integration of Google Fonts is already subject to a legal warning. This tool can help to quickly identify problems, especially with managed wordpress sites.

## Usage
```
Usage: threepwoods-colly [-d 3] [-v] http://website.com
  -d int
        max depth for page visits when following links (default 3)
  -v    verbose output
```

## Results without guarantee

With Consent Management Plattforms preventing code execution and many possible ways to inject resources into a website, there may occur both false positives and negatives. If you find some, please report them with an example.