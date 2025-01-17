package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/hesusruiz/vcbackend/back/handlers"
	"github.com/hesusruiz/vcbackend/back/operations"
	"github.com/hesusruiz/vcbackend/vault"
	"github.com/hesusruiz/vcutils/yaml"

	"flag"
	"log"

	qrcode "github.com/skip2/go-qrcode"

	"github.com/valyala/fasttemplate"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/csrf"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/gofiber/fiber/v2/utils"
	"github.com/gofiber/storage/memory"
	"github.com/gofiber/template/html"
	"go.uber.org/zap"
)

const defaultConfigFile = "configs/server.yaml"
const defaultTemplateDir = "back/views"
const defaultStaticDir = "back/www"
const defaultStoreDriverName = "sqlite3"
const defaultStoreDataSourceName = "file:issuer.sqlite?mode=rwc&cache=shared&_fk=1"
const defaultPassword = "ThePassword"

const corePrefix = "/core/api/v1"
const issuerPrefix = "/issuer/api/v1"
const verifierPrefix = "/verifier/api/v1"
const walletPrefix = "/wallet/api/v1"

var (
	prod       = flag.Bool("prod", false, "Enable prefork in Production")
	configFile = flag.String("config", LookupEnvOrString("CONFIG_FILE", defaultConfigFile), "path to configuration file")
	password   = flag.String("pass", LookupEnvOrString("PASSWORD", defaultPassword), "admin password for the server")
)

type SSIKitConfig struct {
	coreUrl      string
	signatoryUrl string
	auditorUrl   string
	custodianUrl string
	essifUrl     string
}

// Server is the struct holding the state of the server
type Server struct {
	*fiber.App
	cfg           *yaml.YAML
	WebAuthn      *handlers.WebAuthnHandler
	Operations    *operations.Manager
	issuerVault   *vault.Vault
	verifierVault *vault.Vault
	walletvault   *vault.Vault
	issuerDID     string
	verifierDID   string
	logger        *zap.SugaredLogger
	storage       *memory.Storage
	ssiKit        *SSIKitConfig
}

func LookupEnvOrString(key string, defaultVal string) string {
	if val, ok := os.LookupEnv(key); ok {
		return val
	}
	return defaultVal
}

func main() {
	run()
}

func run() {
	var err error

	// Create the server instance
	s := Server{}

	// Read configuration file
	cfg := readConfiguration(*configFile)

	// Create the logger and store in Server so all handlers can use it
	if cfg.String("server.environment") == "production" {
		s.logger = zap.Must(zap.NewProduction()).Sugar()
	} else {
		s.logger = zap.Must(zap.NewDevelopment()).Sugar()
	}
	zap.WithCaller(true)
	defer s.logger.Sync()

	// Parse command-line flags
	flag.Parse()

	// Create the template engine using the templates in the configured directory
	templateDir := cfg.String("server.templateDir", defaultTemplateDir)
	templateEngine := html.New(templateDir, ".html")

	if cfg.String("server.environment") == "development" {
		// Just for development time. Disable when in production
		templateEngine.Reload(true)
	}

	// Define the configuration for Fiber
	fiberCfg := fiber.Config{
		Views:       templateEngine,
		ViewsLayout: "layouts/main",
		Prefork:     *prod,
	}

	// Create a Fiber instance and set it in our Server struct
	s.App = fiber.New(fiberCfg)
	s.cfg = cfg

	// Connect to the different store engines
	s.issuerVault = vault.Must(vault.New(yaml.New(cfg.Map("issuer"))))
	s.verifierVault = vault.Must(vault.New(yaml.New(cfg.Map("verifier"))))
	s.walletvault = vault.Must(vault.New(yaml.New(cfg.Map("wallet"))))

	// Create the issuer and verifier users
	// TODO: the password is only for testing
	s.issuerVault.CreateUserWithKey(cfg.String("issuer.id"), cfg.String("issuer.name"), "legalperson", cfg.String("issuer.password"))
	s.verifierVault.CreateUserWithKey(cfg.String("verifier.id"), cfg.String("verifier.name"), "legalperson", cfg.String("verifier.password"))

	s.ssiKit = fromMap(cfg.Map("ssikit"))

	s.logger.Infof("SSIKit is configured at: %v", s.ssiKit)

	// Create the DIDs for the issuer and verifier
	s.issuerDID, err = operations.SSIKitCreateDID(s.ssiKit.custodianUrl, s.issuerVault, cfg.String("issuer.id"))
	if err != nil {
		panic(err)
	}
	s.logger.Infow("IssuerDID created", "did", s.issuerDID)

	s.verifierDID, err = operations.SSIKitCreateDID(s.ssiKit.custodianUrl, s.verifierVault, cfg.String("verifier.id"))
	if err != nil {
		panic(err)
	}
	s.logger.Infow("VerifierDID created", "did", s.verifierDID)

	// Backend Operations, with its DB connection configuration
	s.Operations = operations.NewManager(cfg)

	// Recover panics from the HTTP handlers so the server continues running
	s.Use(recover.New(recover.Config{EnableStackTrace: true}))

	s.Use(logger.New(logger.Config{
		// TimeFormat: "02-Jan-1985",
		TimeZone: "Europe/Brussels",
	}))

	// CORS
	s.Use(cors.New())

	// CSRF
	csrfHandler := csrf.New(csrf.Config{
		KeyLookup:      "form:_csrf",
		ContextKey:     "csrftoken",
		CookieName:     "csrf_",
		CookieSameSite: "Strict",
		Expiration:     1 * time.Hour,
		KeyGenerator:   utils.UUID,
	})

	// Create a storage entry for logon expiration
	s.storage = memory.New()
	defer s.storage.Close()

	// WebAuthn
	// app.WebAuthn = handlers.NewWebAuthnHandler(app.App, app.Operations, cfg)

	// ##########################
	// Application Home pages
	s.Get("/", s.HandleHome)
	s.Get("/issuer", s.HandleIssuerHome)
	s.Get("/verifier", s.HandleVerifierHome)
	s.Get("/stop", s.HandleStop)

	// ##########################
	// Issuer routes
	issuerRoutes := s.Group(issuerPrefix)

	// Handle new credential
	issuerRoutes.Get("/newcredential", csrfHandler, s.IssuerPageNewCredentialFormDisplay)
	issuerRoutes.Post("/newcredential", csrfHandler, s.IssuerPageNewCredentialFormPost)

	// Display details of a credential
	issuerRoutes.Get("/creddetails/:id", s.IssuerPageCredentialDetails)

	// Display a QR with a URL for retrieving the credential from the server
	issuerRoutes.Get("/displayqrurl/:id", s.IssuerPageDisplayQRURL)

	// Get a list of all credentials
	issuerRoutes.Get("/allcredentials", s.IssuerAPIAllCredentials)

	// Get a credential given its ID
	issuerRoutes.Get("/credential/:id", s.IssuerAPICredential)

	// ###########################
	// Verifier routes
	verifierRoutes := s.Group(verifierPrefix)

	verifierRoutes.Get("/displayqr", s.VerifierPageDisplayQRSIOP)
	verifierRoutes.Get("/loginexpired", s.VerifierPageLoginExpired)
	verifierRoutes.Get("/startsiopsamedevice", s.VerifierPageStartSIOPSameDevice)
	verifierRoutes.Get("/receivecredential/:state", s.VerifierPageReceiveCredential)
	verifierRoutes.Get("/accessprotectedservice", s.VerifierPageAccessProtectedService)

	verifierRoutes.Get("/poll/:state", s.VerifierAPIPoll)
	verifierRoutes.Get("/startsiop", s.VerifierAPIStartSIOP)
	verifierRoutes.Post("/authenticationresponse", s.VerifierAPIAuthenticationResponse)

	// ########################################
	// Wallet routes
	walletRoutes := s.Group(walletPrefix)

	walletRoutes.Get("/selectcredential", s.WalletPageSelectCredential)
	walletRoutes.Get("/sendcredential", s.WalletPageSendCredential)

	// ########################################
	// Core routes
	coreRoutes := s.Group(corePrefix)

	// Create DID
	coreRoutes.Get("/createdid", s.CoreAPICreateDID)
	// List Templates
	coreRoutes.Get("/listcredentialtemplates", s.CoreAPIListCredentialTemplates)
	// Get one template
	coreRoutes.Get("/getcredentialtemplate/:id", s.CoreAPIGetCredentialTemplate)

	// ########################################

	// Setup static files
	s.Static("/static", cfg.String("server.staticDir", defaultStaticDir))

	// Start the server
	log.Fatal(s.Listen(cfg.String("server.listenAddress")))

}

func fromMap(configMap map[string]any) (skc *SSIKitConfig) {
	coreUrl, ok := configMap["coreURL"]
	if !ok {
		panic(errors.New("no_core_url"))
	}
	custodianUrl, ok := configMap["custodianURL"]
	if !ok {
		panic(errors.New("no_custodian_url"))
	}
	signatoryUrl, ok := configMap["signatoryURL"]
	if !ok {
		panic(errors.New("no_signatory_url"))
	}
	essifUrl, ok := configMap["essifURL"]
	if !ok {
		panic(errors.New("no_essif_url"))
	}
	auditorUrl, ok := configMap["auditorURL"]
	if !ok {
		panic(errors.New("no_auditor_url"))
	}
	return &SSIKitConfig{coreUrl: coreUrl.(string), signatoryUrl: signatoryUrl.(string), auditorUrl: auditorUrl.(string), essifUrl: essifUrl.(string), custodianUrl: custodianUrl.(string)}
}

func (s *Server) HandleHome(c *fiber.Ctx) error {

	// Render index
	return c.Render("index", "")
}

func (s *Server) HandleStop(c *fiber.Ctx) error {
	os.Exit(0)
	return nil
}

func (s *Server) HandleIssuerHome(c *fiber.Ctx) error {

	// Get the list of credentials
	credsSummary, err := s.Operations.GetAllCredentials()
	if err != nil {
		return err
	}

	// Render template
	m := fiber.Map{
		"issuerPrefix":   issuerPrefix,
		"verifierPrefix": verifierPrefix,
		"walletPrefix":   walletPrefix,
		"prefix":         issuerPrefix,
		"credlist":       credsSummary,
	}
	return c.Render("issuer_home", m)
}

func (s *Server) HandleVerifierHome(c *fiber.Ctx) error {

	// Get the list of credentials
	credsSummary, err := s.Operations.GetAllCredentials()
	if err != nil {
		return err
	}

	// Render template
	m := fiber.Map{
		"issuerPrefix":   issuerPrefix,
		"verifierPrefix": verifierPrefix,
		"walletPrefix":   walletPrefix,
		"prefix":         verifierPrefix,
		"credlist":       credsSummary,
	}
	return c.Render("verifier_home", m)
}

func (s *Server) IssuerPageDisplayQRURL(c *fiber.Ctx) error {

	// Get the credential ID from the path parameter
	id := c.Params("id")

	// Generate the state that will be used for checking expiration
	state := generateNonce()

	// Create an entry in storage that will expire in 2 minutes
	// The entry is identified by the nonce
	// s.storage.Set(state, []byte("pending"), 2*time.Minute)
	s.storage.Set(state, []byte("pending"), 40*time.Second)

	// QR code for cross-device SIOP
	template := "{{protocol}}://{{hostname}}{{prefix}}/credential/{{id}}?state={{state}}"
	t := fasttemplate.New(template, "{{", "}}")
	str := t.ExecuteString(map[string]interface{}{
		"protocol": c.Protocol(),
		"hostname": c.Hostname(),
		"prefix":   issuerPrefix,
		"id":       id,
		"state":    state,
	})

	// Create the QR
	png, err := qrcode.Encode(str, qrcode.Medium, 256)
	if err != nil {
		return err
	}

	// Convert to a dataURL
	base64Img := base64.StdEncoding.EncodeToString(png)
	base64Img = "data:image/png;base64," + base64Img

	// Render index
	m := fiber.Map{
		"issuerPrefix":   issuerPrefix,
		"verifierPrefix": verifierPrefix,
		"walletPrefix":   walletPrefix,
		"qrcode":         base64Img,
		"state":          state,
	}
	return c.Render("issuer_present_qr", m)
}

func generateNonce() string {
	b := make([]byte, 16)
	io.ReadFull(rand.Reader, b)
	nonce := base64.RawURLEncoding.EncodeToString(b)
	return nonce
}

var sameDevice = false

func (s *Server) VerifierPageDisplayQR(c *fiber.Ctx) error {

	if sameDevice {
		return s.VerifierPageStartSIOPSameDevice(c)
	}

	// Generate the state that will be used for checking expiration
	state := generateNonce()

	// Create an entry in storage that will expire in 2 minutes
	// The entry is identified by the nonce
	// s.storage.Set(state, []byte("pending"), 2*time.Minute)
	s.storage.Set(state, []byte("pending"), 40*time.Second)

	// QR code for cross-device SIOP
	template := "{{protocol}}://{{hostname}}{{prefix}}/startsiop?state={{state}}"
	qrCode1, err := qrCode(template, c.Protocol(), c.Hostname(), verifierPrefix, state)
	if err != nil {
		return err
	}

	// Render index
	m := fiber.Map{
		"issuerPrefix":   issuerPrefix,
		"verifierPrefix": verifierPrefix,
		"walletPrefix":   walletPrefix,
		"qrcode":         qrCode1,
		"prefix":         verifierPrefix,
		"state":          state,
	}
	return c.Render("verifier_present_qr", m)
}

func qrCode(template, protocol, hostname, prefix, state string) (string, error) {

	// Construct the URL to be included in the QR
	t := fasttemplate.New(template, "{{", "}}")
	str := t.ExecuteString(map[string]interface{}{
		"protocol": protocol,
		"hostname": hostname,
		"prefix":   prefix,
		"state":    state,
	})

	// Create the QR
	png, err := qrcode.Encode(str, qrcode.Medium, 256)
	if err != nil {
		return "", err
	}

	// Convert to a dataURL
	base64Img := base64.StdEncoding.EncodeToString(png)
	base64Img = "data:image/png;base64," + base64Img

	return base64Img, nil

}

func (s *Server) VerifierPageDisplayQRSIOP(c *fiber.Ctx) error {

	// Generate the state that will be used for checking expiration
	state := generateNonce()

	// Create an entry in storage that will expire in 2 minutes
	// The entry is identified by the nonce
	// s.storage.Set(state, []byte("pending"), 2*time.Minute)
	s.storage.Set(state, []byte("pending"), 200*time.Second)

	// QR code for cross-device SIOP

	const scope = "dsba.credentials.presentation.PacketDeliveryService"
	const response_type = "vp_token"
	redirect_uri := c.Protocol() + "://" + c.Hostname() + verifierPrefix + "/authenticationresponse"

	template := "openid://?scope={{scope}}" +
		"&response_type={{response_type}}" +
		"&response_mode=post" +
		"&client_id={{client_id}}" +
		"&redirect_uri={{redirect_uri}}" +
		"&state={{state}}" +
		"&nonce={{nonce}}"

	t := fasttemplate.New(template, "{{", "}}")
	str := t.ExecuteString(map[string]interface{}{
		"scope":         scope,
		"response_type": response_type,
		"client_id":     s.verifierDID,
		"redirect_uri":  redirect_uri,
		"state":         state,
		"nonce":         generateNonce(),
	})
	fmt.Println(str)

	// Create the QR
	png, err := qrcode.Encode(str, qrcode.Medium, 256)
	if err != nil {
		return err
	}

	// Convert to a dataURL
	base64Img := base64.StdEncoding.EncodeToString(png)
	base64Img = "data:image/png;base64," + base64Img

	// Render the page
	m := fiber.Map{
		"issuerPrefix":   issuerPrefix,
		"verifierPrefix": verifierPrefix,
		"walletPrefix":   walletPrefix,
		"qrcode":         base64Img,
		"prefix":         verifierPrefix,
		"state":          state,
	}
	return c.Render("verifier_present_qr", m)
}

func (s *Server) VerifierAPIPoll(c *fiber.Ctx) error {

	// get the state
	state := c.Params("state")

	// Check if session still pending
	status, _ := s.storage.Get(state)
	if len(status) == 0 {
		return c.SendString("expired")
	} else {
		return c.SendString(string(status))
	}

}

func (s *Server) VerifierPageLoginExpired(c *fiber.Ctx) error {
	m := fiber.Map{
		"prefix": verifierPrefix,
	}
	return c.Render("verifier_loginexpired", m)
}

func (s *Server) VerifierPageStartSIOPSameDevice(c *fiber.Ctx) error {

	state := c.Query("state")

	const scope = "dsba.credentials.presentation.PacketDeliveryService"
	const response_type = "vp_token"
	redirect_uri := c.Protocol() + "://" + c.Hostname() + verifierPrefix + "/authenticationresponse"

	// template := "https://hesusruiz.github.io/faster/?scope={{scope}}" +
	// 	"&response_type={{response_type}}" +
	// 	"&response_mode=post" +
	// 	"&client_id={{client_id}}" +
	// 	"&redirect_uri={{redirect_uri}}" +
	// 	"&state={{state}}" +
	// 	"&nonce={{nonce}}"

	walletUri := c.Protocol() + "://" + c.Hostname() + walletPrefix + "/selectcredential"
	template := walletUri + "/?scope={{scope}}" +
		"&response_type={{response_type}}" +
		"&response_mode=post" +
		"&client_id={{client_id}}" +
		"&redirect_uri={{redirect_uri}}" +
		"&state={{state}}" +
		"&nonce={{nonce}}"

	t := fasttemplate.New(template, "{{", "}}")
	str := t.ExecuteString(map[string]interface{}{
		"scope":         scope,
		"response_type": response_type,
		"client_id":     s.verifierDID,
		"redirect_uri":  redirect_uri,
		"state":         state,
		"nonce":         generateNonce(),
	})
	fmt.Println(str)

	return c.Redirect(str)
}

func (s *Server) VerifierAPIStartSIOP(c *fiber.Ctx) error {

	// Get the state
	state := c.Query("state")

	const scope = "dsba.credentials.presentation.PacketDeliveryService"
	const response_type = "vp_token"
	redirect_uri := c.Protocol() + "://" + c.Hostname() + verifierPrefix + "/authenticationresponse"

	template := "openid://?scope={{scope}}" +
		"&response_type={{response_type}}" +
		"&response_mode=post" +
		"&client_id={{client_id}}" +
		"&redirect_uri={{redirect_uri}}" +
		"&state={{state}}" +
		"&nonce={{nonce}}"

	t := fasttemplate.New(template, "{{", "}}")
	str := t.ExecuteString(map[string]interface{}{
		"scope":         scope,
		"response_type": response_type,
		"client_id":     s.verifierDID,
		"redirect_uri":  redirect_uri,
		"state":         state,
		"nonce":         generateNonce(),
	})
	fmt.Println(str)

	return c.SendString(str)
}

func (s *Server) VerifierAPIAuthenticationResponse(c *fiber.Ctx) error {

	// Get the state
	state := c.Query("state")

	// We should receive the credential in the body as JSON
	body := c.Body()
	fmt.Println(string(body))

	// Decode into a map
	cred, err := yaml.ParseJson(string(body))
	if err != nil {
		s.logger.Errorw("invalid credential received", zap.Error(err))
		return err
	}

	credential := cred.String("credential")
	// Validate the credential

	// Set the credential in storage, and wait for the polling from client
	s.storage.Set(state, []byte(credential), 10*time.Second)

	return c.SendString("ok")
}

func (s *Server) HandleAuthenticationRequest(c *fiber.Ctx) error {

	// Get the list of credentials
	credsSummary, err := s.Operations.GetAllCredentials()
	if err != nil {
		return err
	}

	// Render template
	m := fiber.Map{
		"prefix":   verifierPrefix,
		"credlist": credsSummary,
	}
	return c.Render("wallet_selectcredential", m)
}

func (s *Server) IssuerAPIAllCredentials(c *fiber.Ctx) error {

	// Get the list of credentials
	credsSummary, err := s.Operations.GetAllCredentials()
	if err != nil {
		return err
	}

	return c.JSON(credsSummary)
}

func (s *Server) IssuerAPICredential(c *fiber.Ctx) error {

	// Get the ID of the credential
	credID := c.Params("id")

	// Get the raw credential from the Vault
	rawCred, err := s.issuerVault.Client.Credential.Get(context.Background(), credID)
	if err != nil {
		return err
	}

	return c.SendString(string(rawCred.Raw))
}

func (s *Server) WalletPageSelectCredential(c *fiber.Ctx) error {

	type authRequest struct {
		Scope         string `query:"scope"`
		Response_mode string `query:"response_mode"`
		Response_type string `query:"response_type"`
		Client_id     string `query:"client_id"`
		Redirect_uri  string `query:"redirect_uri"`
		State         string `query:"state"`
		Nonce         string `query:"nonce"`
	}

	ar := new(authRequest)
	if err := c.QueryParser(ar); err != nil {
		return err
	}

	// Get the list of credentials
	credsSummary, err := s.Operations.GetAllCredentials()
	if err != nil {
		return err
	}

	// Render template
	m := fiber.Map{
		"issuerPrefix":   issuerPrefix,
		"verifierPrefix": verifierPrefix,
		"walletPrefix":   walletPrefix,
		"prefix":         walletPrefix,
		"authRequest":    ar,
		"credlist":       credsSummary,
	}
	return c.Render("wallet_selectcredential", m)
}

func (s *Server) WalletPageSendCredential(c *fiber.Ctx) error {

	// Get the ID of the credential
	credID := c.Query("id")
	s.logger.Info("credID", credID)

	// Get the url where we have to send the credential
	redirect_uri := c.Query("redirect_uri")
	s.logger.Info("redirect_uri", redirect_uri)

	// Get the state nonce
	state := c.Query("state")
	s.logger.Info("state", state)

	// Get the raw credential from the Vault
	// TODO: change to the vault of the wallet without relying on the issuer
	rawCred, err := s.issuerVault.Client.Credential.Get(context.Background(), credID)
	if err != nil {
		return err
	}

	// Prepare to POST the credential to the url, passing the state
	agent := fiber.Post(redirect_uri)
	agent.QueryString("state=" + state)

	// Set the credential in the body of the request
	bodyRequest := fiber.Map{
		"credential": string(rawCred.Raw),
	}
	agent.JSON(bodyRequest)

	// Set content type, both for request and accepted reply
	agent.ContentType("application/json")
	agent.Set("accept", "application/json")

	// Send the request.
	// We are interested only in the success of the request.
	code, _, errors := agent.Bytes()
	if len(errors) > 0 {
		s.logger.Errorw("error sending credential", zap.Errors("errors", errors))
		return fmt.Errorf("error sending credential: %v", errors[0])
	}

	fmt.Println("code:", code)

	// Tell the user that it was OK
	m := fiber.Map{
		"issuerPrefix":   issuerPrefix,
		"verifierPrefix": verifierPrefix,
		"walletPrefix":   walletPrefix,
		"prefix":         verifierPrefix,
		"error":          "",
	}
	if code < 200 || code > 299 {
		m["error"] = fmt.Sprintf("Error calling server: %v", code)
	}
	return c.Render("wallet_credentialsent", m)
}

func (s *Server) VerifierPageReceiveCredential(c *fiber.Ctx) error {

	// Get the state as a path parameter
	state := c.Params("state")

	// get the credential from the storage
	rawCred, _ := s.storage.Get(state)
	if len(rawCred) == 0 {
		// Render an error
		m := fiber.Map{
			"error": "No credential found",
		}
		return c.Render("displayerror", m)
	}

	claims := string(rawCred)

	// Create an access token from the credential
	accessToken, err := s.issuerVault.CreateAccessToken(claims, s.cfg.String("issuer.id"))
	if err != nil {
		return err
	}

	// Set it in a cookie
	cookie := new(fiber.Cookie)
	cookie.Name = "dbsamvf"
	cookie.Value = string(accessToken)
	cookie.Expires = time.Now().Add(1 * time.Hour)

	// Set cookie
	c.Cookie(cookie)

	// Set also the access token in the Authorization field of the response header
	bearer := "Bearer " + string(accessToken)
	c.Set("Authorization", bearer)

	// Render
	m := fiber.Map{
		"issuerPrefix":   issuerPrefix,
		"verifierPrefix": verifierPrefix,
		"walletPrefix":   walletPrefix,
		"claims":         claims,
		"prefix":         verifierPrefix,
	}
	return c.Render("verifier_receivedcredential", m)
}

func (s *Server) VerifierPageAccessProtectedService(c *fiber.Ctx) error {

	var code int
	var returnBody []byte
	var errors []error

	// Get the access token from the cookie
	accessToken := c.Cookies("dbsamvf")

	// Check if the user has configured a protected service to access
	protected := s.cfg.String("verifier.protectedResource.url")
	if len(protected) > 0 {

		// Prepare to GET to the url
		agent := fiber.Get(protected)

		// Set the Authentication header
		agent.Set("Authorization", "Bearer "+accessToken)

		agent.Set("accept", "application/json")
		code, returnBody, errors = agent.Bytes()
		if len(errors) > 0 {
			s.logger.Errorw("error calling SSI Kit", zap.Errors("errors", errors))
			return fmt.Errorf("error calling SSI Kit: %v", errors[0])
		}

	}

	// Render
	m := fiber.Map{
		"issuerPrefix":   issuerPrefix,
		"verifierPrefix": verifierPrefix,
		"walletPrefix":   walletPrefix,
		"accesstoken":    accessToken,
		"protected":      protected,
		"code":           code,
		"returnBody":     string(returnBody),
	}
	return c.Render("verifier_protectedservice", m)
}

// ##########################################
// ##########################################
// New Credential begin

func (s *Server) IssuerPageNewCredentialFormDisplay(c *fiber.Ctx) error {

	// Display the form to enter credential data
	m := fiber.Map{
		"issuerPrefix":   issuerPrefix,
		"verifierPrefix": verifierPrefix,
		"walletPrefix":   walletPrefix,
		"csrftoken":      c.Locals("csrftoken"),
		"prefix":         issuerPrefix,
	}

	return c.Render("issuer_newcredential", m)
}

type NewCredentialForm struct {
	FirstName  string `form:"firstName,omitempty"`
	FamilyName string `form:"familyName,omitempty"`
	Email      string `form:"email,omitempty"`
	Target     string `form:"target,omitempty"`
	Roles      string `form:"roles,omitempty"`
}

func (s *Server) IssuerPageNewCredentialFormPost(c *fiber.Ctx) error {

	// The user submitted the form. Get all the data
	newCred := &NewCredentialForm{}
	if err := c.BodyParser(newCred); err != nil {
		return err
	}

	m := fiber.Map{}

	// Display again the form if there are errors on input
	if newCred.Email == "" || newCred.FirstName == "" || newCred.FamilyName == "" ||
		newCred.Roles == "" || newCred.Target == "" {
		m["Errormessage"] = "Enter all fields"
		return c.Render("issuer_newcredential", m)
	}

	// Convert to the hierarchical map required for the template
	claims := fiber.Map{}

	claims["firstName"] = newCred.FirstName
	claims["familyName"] = newCred.FamilyName
	claims["email"] = newCred.Email

	names := strings.Split(newCred.Roles, ",")
	var roles []map[string]any
	role := map[string]any{
		"target": newCred.Target,
		"names":  names,
	}

	roles = append(roles, role)
	claims["roles"] = roles

	credentialData := fiber.Map{}
	credentialData["credentialSubject"] = claims

	// credID, _, err := srv.Operations.CreateServiceCredential(claims)
	// if err != nil {
	// 	return err
	// }

	// Get the issuer DID
	issuerDID, err := s.issuerVault.GetDIDForUser(s.cfg.String("issuer.id"))
	if err != nil {
		return err
	}

	// Call the issuer of SSI Kit
	agent := fiber.Post(s.ssiKit.signatoryUrl + "/v1/credentials/issue")

	config := fiber.Map{
		"issuerDid":  issuerDID,
		"subjectDid": "did:key:z6Mkfdio1n9SKoZUtKdr9GTCZsRPbwHN8f7rbJghJRGdCt88",
		// "verifierDid": "theVerifier",
		// "issuerVerificationMethod": "string",
		"proofType": "LD_PROOF",
		// "domain":                   "string",
		// "nonce":                    "string",
		// "proofPurpose":             "string",
		// "credentialId":             "string",
		// "issueDate":                "2022-10-06T18:09:14.570Z",
		// "validDate":                "2022-10-06T18:09:14.570Z",
		// "expirationDate":           "2022-10-06T18:09:14.570Z",
		// "dataProviderIdentifier":   "string",
	}

	bodyRequest := fiber.Map{
		"templateId":     "PacketDeliveryService",
		"config":         config,
		"credentialData": credentialData,
	}

	out, _ := json.MarshalIndent(bodyRequest, "", "  ")
	fmt.Println(string(out))

	agent.JSON(bodyRequest)
	agent.ContentType("application/json")
	agent.Set("accept", "application/json")
	_, returnBody, errors := agent.Bytes()
	if len(errors) > 0 {
		s.logger.Errorw("error calling SSI Kit", zap.Errors("errors", errors))
		return fmt.Errorf("error calling SSI Kit: %v", errors[0])
	}

	parsed, err := yaml.ParseJson(string(returnBody))
	if err != nil {
		return err
	}

	credentialID := parsed.String("id")
	if len(credentialID) == 0 {
		s.logger.Errorw("id field not found in credential")
		return fmt.Errorf("id field not found in credential")
	}

	// Store credential
	_, err = s.issuerVault.Client.Credential.Create().
		SetID(credentialID).
		SetRaw([]uint8(returnBody)).
		Save(context.Background())
	if err != nil {
		s.logger.Errorw("error storing the credential", zap.Error(err))
		return err
	}

	str := prettyFormatJSON(returnBody)
	fmt.Println(str)

	// Render
	m = fiber.Map{
		"issuerPrefix":   issuerPrefix,
		"verifierPrefix": verifierPrefix,
		"walletPrefix":   walletPrefix,
		"claims":         str,
		"prefix":         issuerPrefix,
	}
	return c.Render("creddetails", m)
}

// New Credential end
// ##########################################
// ##########################################

func (s *Server) IssuerPageCredentialDetails(c *fiber.Ctx) error {

	// Get the ID of the credential
	credID := c.Params("id")

	claims, err := s.Operations.GetCredentialLD(credID)
	if err != nil {
		return err
	}

	// Render
	m := fiber.Map{
		"issuerPrefix":   issuerPrefix,
		"verifierPrefix": verifierPrefix,
		"walletPrefix":   walletPrefix,
		"claims":         claims,
		"prefix":         issuerPrefix,
	}
	return c.Render("creddetails", m)
}

// readConfiguration reads a YAML file and creates an easy-to navigate structure
func readConfiguration(configFile string) *yaml.YAML {
	var cfg *yaml.YAML
	var err error

	cfg, err = yaml.ParseYamlFile(configFile)
	if err != nil {
		fmt.Printf("Config file not found, exiting\n")
		panic(err)
	}
	return cfg
}

// ##########################################
// ##########################################
//              APIS
// ##########################################
// ##########################################

// DID handling
func (srv *Server) CoreAPICreateDID(c *fiber.Ctx) error {

	// body := c.Body()

	// Call the SSI Kit
	agent := fiber.Post(srv.ssiKit.custodianUrl + "/did/create")
	bodyRequest := fiber.Map{
		"method": "key",
	}
	agent.JSON(bodyRequest)
	agent.ContentType("application/json")
	agent.Set("accept", "application/json")
	_, returnBody, errors := agent.Bytes()
	if len(errors) > 0 {
		srv.logger.Errorw("error calling SSI Kit", zap.Errors("errors", errors))
		return fmt.Errorf("error calling SSI Kit: %v", errors[0])
	}

	c.Set("Content-Type", "application/json")
	return c.Send(returnBody)

}

func (srv *Server) CoreAPIListCredentialTemplates(c *fiber.Ctx) error {

	// Call the SSI Kit
	agent := fiber.Get(srv.ssiKit.signatoryUrl + "/v1/templates")
	agent.Set("accept", "application/json")
	_, returnBody, errors := agent.Bytes()
	if len(errors) > 0 {
		srv.logger.Errorw("error calling SSI Kit", zap.Errors("errors", errors))
		return fmt.Errorf("error calling SSI Kit: %v", errors[0])
	}

	c.Set("Content-Type", "application/json")
	return c.Send(returnBody)

}

func (srv *Server) CoreAPIGetCredentialTemplate(c *fiber.Ctx) error {

	id := c.Params("id")
	if len(id) == 0 {
		return fmt.Errorf("no template id specified")
	}

	// Call the SSI Kit
	agent := fiber.Get(srv.ssiKit.signatoryUrl + "/v1/templates/" + id)
	agent.Set("accept", "application/json")
	_, returnBody, errors := agent.Bytes()
	if len(errors) > 0 {
		srv.logger.Errorw("error calling SSI Kit", zap.Errors("errors", errors))
		return fmt.Errorf("error calling SSI Kit: %v", errors[0])
	}

	c.Set("Content-Type", "application/json")
	return c.Send(returnBody)

}

func prettyFormatJSON(in []byte) string {
	decoded := &fiber.Map{}
	json.Unmarshal(in, decoded)
	out, _ := json.MarshalIndent(decoded, "", "  ")
	return string(out)
}
