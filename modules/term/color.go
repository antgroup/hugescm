package term

func (v Level) Red(s string) string {
	switch v {
	case Level16M:
		// #f43b47
		return "\x1b[38;2;244;59;71m" + s + "\x1b[0m"
	case Level256:
		// \e[0;31m	Red
		return "\x1b[31m" + s + "\x1b[0m"
	default:
	}
	return s
}

func (v Level) Green(s string) string {
	switch v {
	case Level16M:
		// #43e97a
		return "\x1b[38;2;67;233;123m" + s + "\x1b[0m"
	case Level256:
		// \e[0;32m	Green
		return "\x1b[32m" + s + "\x1b[0m"
	default:
	}
	return s
}

func (v Level) Yellow(s string) string {
	switch v {
	case Level16M:
		// #fee240
		return "\x1b[38;2;254;225;64m" + s + "\x1b[0m"
	case Level256:
		// \e[0;33m	Yellow
		return "\x1b[33m" + s + "\x1b[0m"
	default:
	}
	return s
}

func (v Level) Blue(s string) string {
	switch v {
	case Level16M:
		// #00c8ff
		return "\x1b[38;2;0;201;255m" + s + "\x1b[0m"
	case Level256:
		// \e[0;34m	Blue
		return "\x1b[34m" + s + "\x1b[0m"
	default:
	}
	return s
}

func (v Level) Purple(s string) string {
	switch v {
	case Level16M:
		// #7028e4
		return "\x1b[38;2;112;40;228m" + s + "\x1b[0m"
	case Level256:
		// \e[0;35m	Purple
		return "\x1b[35m" + s + "\x1b[0m"
	default:
	}
	return s
}
