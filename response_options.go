package gldap

type responseOptions struct {
	withDiagnosticMessage string
	withMatchedDN         string
	withResponseCode      *int
	withApplicationCode   *int
	withAttributes        map[string][]string
}

func responseDefaults() responseOptions {
	return responseOptions{
		withMatchedDN:         "Unused",
		withDiagnosticMessage: "Unused",
	}
}

func getResponseOpts(opt ...Option) responseOptions {
	opts := responseDefaults()
	applyOpts(&opts, opt...)
	return opts
}

// WithDiagnosticMessage provides an optional diagnostic message for the
// response.
func WithDiagnosticMessage(msg string) Option {
	return func(o interface{}) {
		if o, ok := o.(*responseOptions); ok {
			o.withDiagnosticMessage = msg
		}
	}
}

// WithMatchedDN provides an optional match DN for the response.
func WithMatchedDN(dn string) Option {
	return func(o interface{}) {
		if o, ok := o.(*responseOptions); ok {
			o.withMatchedDN = dn
		}
	}
}

// WithResponseCode specifies the ldap response code.  For a list of valid codes
// see:
// https://github.com/go-ldap/ldap/blob/13008e4c5260d08625b65eb1f172ae909152b751/v3/error.go#L11
func WithResponseCode(code int) Option {
	return func(o interface{}) {
		if o, ok := o.(*responseOptions); ok {
			o.withResponseCode = &code
		}
	}
}

// WithApplicationCode specifies the ldap application code.  For a list of valid codes
// for a list of supported application codes see:
// https://github.com/go-ldap/ldap/blob/13008e4c5260d08625b65eb1f172ae909152b751/v3/ldap.go#L11
func WithApplicationCode(ldapApplicationCode int) Option {
	return func(o interface{}) {
		if o, ok := o.(*responseOptions); ok {
			o.withApplicationCode = &ldapApplicationCode
		}
	}
}

// WithAttributes specifies optional attributes for a response entry
func WithAttributes(attributes map[string][]string) Option {
	return func(o interface{}) {
		if o, ok := o.(*responseOptions); ok {
			o.withAttributes = attributes
		}
	}
}
