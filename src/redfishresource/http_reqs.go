package domain

type HTTPReqType int

const (
	HTTP_GET HTTPReqType = iota
	HTTP_POST
	HTTP_PATCH
	HTTP_PUT
	HTTP_DELETE
	HTTP_HEAD
	HTTP_OPTIONS
	HTTP_UNKNOWN
)

func MapStringToHTTPReq(req string) HTTPReqType {
	switch req {
	case "GET":
		return HTTP_GET
	case "POST":
		return HTTP_POST
	case "PATCH":
		return HTTP_PATCH
	case "PUT":
		return HTTP_PUT
	case "DELETE":
		return HTTP_DELETE
	case "HEAD":
		return HTTP_HEAD
	case "OPTIONS":
		return HTTP_OPTIONS
	default:
		return HTTP_UNKNOWN
	}
}

func MapHTTPReqToString(req HTTPReqType) string {
	switch req {
	case HTTP_GET:
		return "GET"
	case HTTP_POST:
		return "POST"
	case HTTP_PATCH:
		return "PATCH"
	case HTTP_PUT:
		return "PUT"
	case HTTP_DELETE:
		return "DELETE"
	case HTTP_HEAD:
		return "HEAD"
	case HTTP_OPTIONS:
		return "OPTIONS"
	default:
		return "UNKNOWN"
	}
}
