package tiime

// App identifies the Tiime web application the server acts as. The client and
// expert applications share the API and the Auth0 tenant, but each has its own
// OAuth client, origin and application headers.
type App struct {
	// Header is the value of the tiime-app header.
	Header string
	// Version is the default value of the tiime-app-version header.
	Version string
	// ClientID is the public Auth0 client of the web application.
	ClientID string
	// Origin is the web origin registered for the Auth0 client.
	Origin string
	// LoginFlowOnly is set when the Auth0 client refuses the password-realm
	// grant: every login then goes through the Universal Login pages.
	LoginFlowOnly bool
}

var (
	// ClientApp is apps.tiime.fr, the application of Tiime customers.
	ClientApp = App{
		Header:   "tiime",
		Version:  "4.40.6",
		ClientID: "iEbsbe3o66gcTBfGRa012kj1Rb6vjAND",
		Origin:   "https://apps.tiime.fr",
	}

	// ExpertApp is expert.tiime.fr, the application of accounting firms.
	ExpertApp = App{
		Header:        "tiime-pro",
		Version:       "5.66.0:5a96b380556a7369261700f2eaa418039affc6bc",
		ClientID:      "ExMoxAnc9Y8BstgApTW7VpiUh63gmPCd",
		Origin:        "https://expert.tiime.fr",
		LoginFlowOnly: true,
	}
)

func (a App) redirectURI() string {
	return a.Origin + "/auth-callback"
}
