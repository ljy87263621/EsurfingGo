package main

import (
	"bufio"
	"errors"
	"esurfing/network"
	"esurfing/utils"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

// xmlEscape escapes special XML characters in user input.
func xmlEscape(s string) string {
	r := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", "\"", "&quot;", "'", "&apos;")
	return r.Replace(s)
}

// Client manages the authentication lifecycle.
type Client struct {
	options       Options
	states        *States
	session       *Session
	httpClient    *http.Client
	keepURL       string
	termURL       string
	keepRetry     string
	retryCount    int
	tick          int64
	lastStatus    network.ConnectivityStatus
	statusChanged bool
}

// Options holds login credentials.
type Options struct {
	LoginUser       string
	LoginPassword   string
	SMSCode         string
	SMSCodeProvider SMSCodeProvider
}

func NewClient(opts Options, states *States, session *Session, baseTransport ...http.RoundTripper) *Client {
	httpClient := network.NewHTTPClient(states, baseTransport...)
	return &Client{
		options:       opts,
		states:        states,
		session:       session,
		httpClient:    httpClient,
		lastStatus:    -1, // force first log
		statusChanged: true,
	}
}

// Run starts the main client loop.
func (c *Client) Run() {
	for c.states.IsRunning() {
		if c.session.IsInitialized() && c.states.IsLogged() {
			nowMs := time.Now().UnixMilli()
			retryMs, err := strconv.ParseInt(c.keepRetry, 10, 64)
			if err == nil && retryMs > 0 && nowMs-c.tick >= retryMs*1000 {
				log.Println("[Client] Sending heartbeat...")
				if err := c.heartbeat(c.states.GetTicket()); err != nil {
					log.Printf("[Client] Heartbeat error: %v", err)
					c.states.SetLogged(false)
					c.statusChanged = true
					continue
				}
				log.Printf("[Client] Heartbeat OK, next retry: %ss", c.keepRetry)
				c.tick = time.Now().UnixMilli()
			}
			time.Sleep(1 * time.Second)
			continue
		}

		status := network.DetectConfigWithClient(c.httpClient, c.states, c.statusChanged)
		c.statusChanged = status.Status != c.lastStatus
		c.lastStatus = status.Status

		switch status.Status {
		case network.StatusSuccess:
			if !(c.session.IsInitialized() && c.states.IsLogged()) && c.statusChanged {
				log.Println("[Client] Network is connected.")
			}
			time.Sleep(1 * time.Second)

		case network.StatusRequireAuthorization:
			if c.statusChanged {
				log.Println("[Client] Network requires authorization, starting auth flow...")
			}
			c.states.SetLogged(false)
			// Apply detected config to states
			c.states.SetAuthURL(status.AuthURL)
			c.states.SetTicketURL(status.TicketURL)
			c.states.SetUserIP(status.UserIP)
			c.states.SetAcIP(status.AcIP)
			if status.SchoolID != "" {
				c.states.SetSchoolID(status.SchoolID)
			}
			if status.Domain != "" {
				c.states.SetDomain(status.Domain)
			}
			if status.Area != "" {
				c.states.SetArea(status.Area)
			}
			if len(status.ExtraCfgURL) > 0 {
				c.states.SetExtraCfgURL(status.ExtraCfgURL)
			}
			c.authorization()
			c.statusChanged = true // force log after auth attempt

		case network.StatusRequestError:
			if c.statusChanged {
				log.Println("[Client] Request Error - see detailed logs above")
			}
			time.Sleep(5 * time.Second)
		}
	}
}

func (c *Client) authorization() {
	var code string
	if strings.TrimSpace(c.options.SMSCode) == "" {
		code = c.checkSMSVerify()
		if !c.states.IsRunning() {
			return
		}
	} else {
		code = c.options.SMSCode
	}

	if code != "" {
		log.Println("[Client] SMS verification code provided.")
	}

	if err := c.initSession(); err != nil {
		var unsupportedErr *UnsupportedAlgorithmError
		if errors.As(err, &unsupportedErr) {
			c.retryCount++
			if c.retryCount >= 5 {
				log.Printf("[Client] Unsupported session algorithm %q. A protocol/client update may be required.", unsupportedErr.AlgoID)
				c.states.SetRunning(false)
			} else {
				log.Printf("[Client] Unsupported session algorithm %q; retry %d/5.", unsupportedErr.AlgoID, c.retryCount)
			}
		} else {
			c.retryCount = 0
			log.Printf("[Client] Session initialization temporarily failed: %v; will retry.", err)
		}
		return
	}

	c.retryCount = 0
	c.states.SetAlgoID(c.session.GetAlgoID())
	log.Printf("[Client] Algo Id: %s", c.session.GetAlgoID())
	log.Printf("[Client] Client ID: %s", c.states.GetClientID())
	log.Printf("[Client] Client IP: %s", c.states.GetUserIP())
	log.Printf("[Client] AC IP: %s", c.states.GetAcIP())
	log.Printf("[Client] MAC: %s", c.states.GetMacAddress())
	log.Printf("[Client] SchoolID: %s, Domain: %s, Area: %s", c.states.GetSchoolID(), c.states.GetDomain(), c.states.GetArea())

	ticket, err := c.getTicket()
	if err != nil {
		log.Printf("Get ticket error: %v", err)
		return
	}
	c.states.SetTicket(ticket)
	log.Printf("[Client] Ticket acquired (%s)", previewForLog(c.states.GetTicket()))

	if err := c.login(code); err != nil {
		log.Printf("Login error: %v", err)
		return
	}

	if c.keepURL == "" {
		log.Println("[Client] KeepUrl is empty, login may have failed.")
		c.session.Free()
		c.states.SetRunning(false)
		return
	}

	c.tick = time.Now().UnixMilli()
	c.states.SetLogged(true)
	log.Println("[Client] The login has been authorized.")
}

func (c *Client) checkSMSVerify() string {
	extraCfg := c.states.GetExtraCfgURL()
	if network.CheckVerifyCodeStatus(c.states, c.httpClient, c.options.LoginUser, extraCfg) &&
		network.GetVerifyCode(c.states, c.httpClient, c.options.LoginUser, extraCfg) {
		log.Println("This login requires a SMS verification code.")
		if c.options.SMSCodeProvider != nil {
			code, ok := c.options.SMSCodeProvider.Wait()
			if !ok {
				return ""
			}
			return code
		}
		reader := bufio.NewReader(os.Stdin)
		for {
			fmt.Print("Input Code: ")
			input, _ := reader.ReadString('\n')
			code := strings.TrimSpace(input)
			if code != "" {
				return code
			}
		}
	}
	return ""
}

func (c *Client) initSession() error {
	log.Printf("[Client] Initializing session, TicketURL supplied (%s), AlgoID: %s", previewForLog(c.states.GetTicketURL()), c.states.GetAlgoID())
	c.session.Free()
	body, err := network.PostRaw(c.httpClient, ticketURLWithClientParams(c.states.GetTicketURL(), c.states.GetAcIP(), c.states.GetUserIP()), c.states.GetAlgoID(), c.states)
	if err != nil {
		return fmt.Errorf("request session ZSM: %w", err)
	}
	log.Printf("[Client] Session ZSM response: %d bytes", len(body))
	if err := c.session.Initialize(body); err != nil {
		return fmt.Errorf("parse session ZSM: %w", err)
	}
	log.Printf("[Client] Session initialized: %v, AlgoID: %s", c.session.IsInitialized(), c.session.GetAlgoID())
	return nil
}

func (c *Client) getTicket() (string, error) {
	payload := fmt.Sprintf(`<?xml version="1.0" encoding="utf-8"?>
<request>
    <user-agent>CCTP/WinSVR5/1068</user-agent>
    <client-id>%s</client-id>
    <local-time>%s</local-time>
    <host-name>%s</host-name>
    <ipv4>%s</ipv4>
    <ipv6></ipv6>
    <mac>%s</mac>
    <ostag>%s</ostag>
    <gwip>%s</gwip>
</request>`,
		c.states.GetClientID(),
		utils.GetTime(),
		HostName,
		c.states.GetUserIP(),
		c.states.GetMacAddress(),
		HostName,
		c.states.GetAcIP(),
	)

	log.Printf("[Client] getTicket payload prepared (%s)", previewForLog(payload))
	encrypted, err := c.session.Encrypt(payload)
	if err != nil {
		return "", fmt.Errorf("encrypt ticket payload: %w", err)
	}
	log.Printf("[Client] getTicket encrypted payload prepared (%s)", previewForLog(encrypted))
	data, err := network.Post(c.httpClient, ticketURLWithClientParams(c.states.GetTicketURL(), c.states.GetAcIP(), c.states.GetUserIP()), encrypted, c.states, nil)
	if err != nil {
		return "", err
	}
	log.Printf("[Client] getTicket raw response received (%s)", previewForLog(data))

	decrypted, err := c.session.Decrypt(data)
	if err != nil {
		return "", fmt.Errorf("decrypt ticket response: %w", err)
	}
	log.Printf("[Client] getTicket response decrypted (%s)", previewForLog(decrypted))
	ticket := extractXMLTag(decrypted, "ticket")
	return ticket, nil
}

func ticketURLWithClientParams(raw, acIP, userIP string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	q := u.Query()
	if q.Get("wlanacip") == "" && acIP != "" {
		q.Set("wlanacip", acIP)
	}
	if q.Get("wlanuserip") == "" && userIP != "" {
		q.Set("wlanuserip", userIP)
	}
	if q.Get("clientip") == "" {
		q.Set("clientip", "1")
	}
	u.RawQuery = q.Encode()
	return u.String()
}

func (c *Client) login(code string) error {
	verify := ""
	if strings.TrimSpace(code) != "" {
		verify = "<verify>" + xmlEscape(code) + "</verify>"
	}

	payload := fmt.Sprintf(`<?xml version="1.0" encoding="utf-8"?>
<request>
    <user-agent>CCTP/WinSVR5/1068</user-agent>
    <client-id>%s</client-id>
    <ticket>%s</ticket>
    <local-time>%s</local-time>
    <userid>%s</userid>
    <passwd>%s</passwd>
    %s
</request>`,
		c.states.GetClientID(),
		c.states.GetTicket(),
		utils.GetTime(),
		xmlEscape(c.options.LoginUser),
		xmlEscape(c.options.LoginPassword),
		verify,
	)

	log.Printf("[Client] login payload prepared (%s)", previewForLog(payload))
	encrypted, err := c.session.Encrypt(payload)
	if err != nil {
		return fmt.Errorf("encrypt login payload: %w", err)
	}
	log.Printf("[Client] login encrypted payload prepared (%s)", previewForLog(encrypted))
	data, err := network.Post(c.httpClient, c.states.GetAuthURL(), encrypted, c.states, nil)
	if err != nil {
		return err
	}
	log.Printf("[Client] login raw response received (%s)", previewForLog(data))

	decrypted, err := c.session.Decrypt(data)
	if err != nil {
		return fmt.Errorf("decrypt login response: %w", err)
	}
	log.Printf("[Client] login response decrypted (%s)", previewForLog(decrypted))
	c.keepURL = extractXMLTag(decrypted, "keep-url")
	c.termURL = extractXMLTag(decrypted, "term-url")
	c.keepRetry = extractXMLTag(decrypted, "keep-retry")

	log.Printf("Keep Url received (%s)", previewForLog(c.keepURL))
	log.Printf("Term Url received (%s)", previewForLog(c.termURL))
	log.Printf("Keep Retry: %s", c.keepRetry)

	return nil
}

func (c *Client) heartbeat(ticket string) error {
	payload := fmt.Sprintf(`<?xml version="1.0" encoding="utf-8"?>
<request>
    <user-agent>CCTP/WinSVR5/1068</user-agent>
    <client-id>%s</client-id>
    <local-time>%s</local-time>
    <host-name>%s</host-name>
    <ipv4>%s</ipv4>
    <ticket>%s</ticket>
    <ipv6></ipv6>
    <mac>%s</mac>
    <ostag>%s</ostag>
</request>`,
		c.states.GetClientID(),
		utils.GetTime(),
		HostName,
		c.states.GetUserIP(),
		ticket,
		c.states.GetMacAddress(),
		HostName,
	)

	encrypted, err := c.session.Encrypt(payload)
	if err != nil {
		return fmt.Errorf("encrypt heartbeat payload: %w", err)
	}
	data, err := network.Post(c.httpClient, c.keepURL, encrypted, c.states, nil)
	if err != nil {
		return err
	}

	decrypted, err := c.session.Decrypt(data)
	if err != nil {
		return fmt.Errorf("decrypt heartbeat response: %w", err)
	}
	interval := extractXMLTag(decrypted, "interval")
	if interval != "" {
		c.keepRetry = interval
	}
	return nil
}

// Term sends the termination request to log out.
func (c *Client) Term() {
	payload := fmt.Sprintf(`<?xml version="1.0" encoding="utf-8"?>
<request>
    <user-agent>CCTP/WinSVR5/1068</user-agent>
    <client-id>%s</client-id>
    <local-time>%s</local-time>
    <host-name>%s</host-name>
    <ipv4>%s</ipv4>
    <ticket>%s</ticket>
    <ipv6></ipv6>
    <mac>%s</mac>
    <ostag>%s</ostag>
</request>`,
		c.states.GetClientID(),
		utils.GetTime(),
		HostName,
		c.states.GetUserIP(),
		c.states.GetTicket(),
		c.states.GetMacAddress(),
		HostName,
	)

	encrypted, err := c.session.Encrypt(payload)
	if err != nil {
		log.Printf("encrypt term payload: %v", err)
		return
	}
	_, _ = network.Post(c.httpClient, c.termURL, encrypted, c.states, nil)
}

// extractXMLTag is a simple XML tag value extractor that also strips CDATA wrappers.
func extractXMLTag(xml, tag string) string {
	startTag := "<" + tag + ">"
	endTag := "</" + tag + ">"
	start := strings.Index(xml, startTag)
	if start == -1 {
		return ""
	}
	start += len(startTag)
	end := strings.Index(xml[start:], endTag)
	if end == -1 {
		return ""
	}
	val := xml[start : start+end]
	// Strip CDATA wrapper if present
	const cdataPrefix = "<![CDATA["
	const cdataSuffix = "]]>"
	if strings.HasPrefix(val, cdataPrefix) && strings.HasSuffix(val, cdataSuffix) {
		val = val[len(cdataPrefix) : len(val)-len(cdataSuffix)]
	}
	return val
}
