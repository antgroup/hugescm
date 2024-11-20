package kong

var (
	W = func(s string) string {
		return s
	}
)

func BindW(w func(s string) string) {
	W = w
}
