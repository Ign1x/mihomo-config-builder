package util

import "net/url"

func RedactURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return "<redacted-url>"
	}
	q := u.Query()
	for k := range q {
		q.Set(k, "REDACTED")
	}
	u.RawQuery = q.Encode()
	if u.User != nil {
		u.User = url.User("REDACTED")
	}
	return u.String()
}
