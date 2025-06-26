package mime

import "slices"

// https://github.com/chromium/chromium/blob/main/third_party/blink/common/mime_util/mime_util.cc
var (
	// These types are excluded from the logic that allows all text/ types because
	// while they are technically text, it's very unlikely that a user expects to
	// see them rendered in text form.
	UnsupportedTextTypes = []string{
		"text/calendar",
		"text/x-calendar",
		"text/x-vcalendar",
		"text/vcalendar",
		"text/vcard",
		"text/x-vcard",
		"text/directory",
		"text/ldif",
		"text/qif",
		"text/x-qif",
		"text/x-csv",
		"text/x-vcf",
		"text/rtf",
		"text/comma-separated-values",
		"text/csv",
		"text/tab-separated-values",
		"text/tsv",
		"text/ofx",                         // https://crbug.com/162238
		"text/vnd.sun.j2me.app-descriptor", // https://crbug.com/176450
		"text/x-ms-iqy",                    // https://crbug.com/1054863
		"text/x-ms-odc",                    // https://crbug.com/1054863
		"text/x-ms-rqy",                    // https://crbug.com/1054863
		"text/x-ms-contact",                // https://crbug.com/1054863
	}
	SupportedNonImageTypes = []string{
		"image/svg+xml", // SVG is text-based XML, even though it has an image/
		// type
		"application/xml", "application/atom+xml", "application/rss+xml",
		"application/xhtml+xml", "application/json",
		"message/rfc822",    // For MHTML support.
		"multipart/related", // For MHTML support.
		"multipart/x-mixed-replace",
		// Note: ADDING a new type here will probably render it AS HTML. This can
		// result in cross site scripting.
	}
)

func DetectAny(in []byte) *MIME {
	// Frozen: Do not restore this code yet.
	// https://github.com/gabriel-vasile/mimetype/issues/680
	data := slices.Clone(in)
	return root.match(data, uint32(len(data)))
}

func (m *MIME) Sanitize() string {
	return m.mime
}
