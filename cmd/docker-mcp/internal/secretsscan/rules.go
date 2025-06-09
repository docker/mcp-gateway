package secretsscan

import (
	"fmt"
	"regexp"
	"strings"
	"sync"
)

const (
	quote     = `["']?`
	connect   = `\s*(:|=>|=)?\s*`
	endSecret = `[.,]?(\s+|$)`
	startWord = "([^0-9a-zA-Z]|^)"
	aws       = `aws_?`
)

type rule struct {
	expression string
	keywords   []string
}

func (r *rule) matches(content string) bool {
	lower := strings.ToLower(content)

	for _, kw := range r.keywords {
		if strings.Contains(lower, strings.ToLower(kw)) {
			if regexp.MustCompile(r.expression).MatchString(content) {
				return true
			}
		}
	}

	return false
}

func withoutWordPrefix(str string) string {
	return fmt.Sprintf("%s(%s)", startWord, str)
}

// Patterns copied from github.com/aquasecurity/trivy/pkg/fanal/secret/builtin-rules.go
var rules = sync.OnceValue(func() []rule {
	return []rule{
		{
			// aws-access-key-id
			expression: withoutWordPrefix(fmt.Sprintf(`(?P<secret>(A3T[A-Z0-9]|AKIA|AGPA|AidA|AROA|AIPA|ANPA|ANVA|ASIA)[A-Z0-9]{16})%s%s`, quote, endSecret)),
			keywords:   []string{"AKIA", "AGPA", "AidA", "AROA", "AIPA", "ANPA", "ANVA", "ASIA"},
		},
		{
			// aws-secret-access-key
			expression: fmt.Sprintf(`(?i)%s%s(sec(ret)?)?_?(access)?_?key%s%s%s(?P<secret>[A-Za-z0-9\/\+=]{40})%s%s`, quote, aws, quote, connect, quote, quote, endSecret),
			keywords:   []string{"key"},
		},
		{
			// github-pat
			expression: withoutWordPrefix(`?P<secret>ghp_[0-9a-zA-Z]{36}`),
			keywords:   []string{"ghp_"},
		},
		{
			// github-oauth
			expression: withoutWordPrefix(`?P<secret>gho_[0-9a-zA-Z]{36}`),
			keywords:   []string{"gho_"},
		},
		{
			// github-app-token
			expression: withoutWordPrefix(`?P<secret>(ghu|ghs)_[0-9a-zA-Z]{36}`),
			keywords:   []string{"ghu_", "ghs_"},
		},
		{
			// github-refresh-token
			expression: withoutWordPrefix(`?P<secret>ghr_[0-9a-zA-Z]{76}`),
			keywords:   []string{"ghr_"},
		},
		{
			// github-fine-grained-pat
			expression: `github_pat_[a-zA-Z0-9]{22}_[a-zA-Z0-9]{59}`,
			keywords:   []string{"github_pat_"},
		},
		{
			// gitlab-pat
			expression: withoutWordPrefix(`?P<secret>glpat-[0-9a-zA-Z\-\_]{20}`),
			keywords:   []string{"glpat-"},
		},
		{
			// hugging-face-access-token
			expression: withoutWordPrefix(`?P<secret>hf_[A-Za-z0-9]{34,40}`),
			keywords:   []string{"hf_"},
		},
		{
			// private-key
			expression: `(?i)-----\s*?BEGIN[ A-Z0-9_-]*?PRIVATE KEY( BLOCK)?\s*?-----[\s]*?(?P<secret>[A-Za-z0-9=+/\\\r\n][A-Za-z0-9=+/\\\s]+)[\s]*?-----\s*?END[ A-Z0-9_-]*? PRIVATE KEY( BLOCK)?\s*?-----`,
			keywords:   []string{"-----"},
		},
		{
			// shopify-token
			expression: `shp(ss|at|ca|pa)_[a-fA-F0-9]{32}`,
			keywords:   []string{"shpss_", "shpat_", "shpca_", "shppa_"},
		},
		{
			// slack-access-token
			expression: withoutWordPrefix(`?P<secret>xox[baprs]-([0-9a-zA-Z]{10,48})`),
			keywords:   []string{"xoxb-", "xoxa-", "xoxp-", "xoxr-", "xoxs-"},
		},
		{
			// stripe-publishable-token
			expression: withoutWordPrefix(`?P<secret>(?i)pk_(test|live)_[0-9a-z]{10,32}`),
			keywords:   []string{"pk_test_", "pk_live_"},
		},
		{
			// stripe-secret-token
			expression: withoutWordPrefix(`?P<secret>(?i)sk_(test|live)_[0-9a-z]{10,32}`),
			keywords:   []string{"sk_test_", "sk_live_"},
		},
		{
			// pypi-upload-token
			expression: `pypi-AgEIcHlwaS5vcmc[A-Za-z0-9\-_]{50,1000}`,
			keywords:   []string{"pypi-AgEIcHlwaS5vcmc"},
		},
		{
			// gcp-service-account
			expression: `\"type\": \"service_account\"`,
			keywords:   []string{"\"type\": \"service_account\""},
		},
		{
			// heroku-api-key
			expression: ` (?i)(?P<key>heroku[a-z0-9_ .\-,]{0,25})(=|>|:=|\|\|:|<=|=>|:).{0,5}['\"](?P<secret>[0-9A-F]{8}-[0-9A-F]{4}-[0-9A-F]{4}-[0-9A-F]{4}-[0-9A-F]{12})['\"]`,
			keywords:   []string{"heroku"},
		},
		{
			// slack-web-hook
			expression: `https:\/\/hooks.slack.com\/services\/[A-Za-z0-9+\/]{44,48}`,
			keywords:   []string{"hooks.slack.com"},
		},
		{
			// twilio-api-key
			expression: `SK[0-9a-fA-F]{32}`,
			keywords:   []string{"SK"},
		},
		{
			// age-secret-key
			expression: `AGE-SECRET-KEY-1[QPZRY9X8GF2TVDW0S3JN54KHCE6MUA7L]{58}`,
			keywords:   []string{"AGE-SECRET-KEY-1"},
		},
		{
			// facebook-token
			expression: `(?i)(?P<key>facebook[a-z0-9_ .\-,]{0,25})(=|>|:=|\|\|:|<=|=>|:).{0,5}['\"](?P<secret>[a-f0-9]{32})['\"]`,
			keywords:   []string{"facebook"},
		},
		{
			// twitter-token
			expression: `(?i)(?P<key>twitter[a-z0-9_ .\-,]{0,25})(=|>|:=|\|\|:|<=|=>|:).{0,5}['\"](?P<secret>[a-f0-9]{35,44})['\"]`,
			keywords:   []string{"twitter"},
		},
		{
			// adobe-client-id
			expression: `(?i)(?P<key>adobe[a-z0-9_ .\-,]{0,25})(=|>|:=|\|\|:|<=|=>|:).{0,5}['\"](?P<secret>[a-f0-9]{32})['\"]`,
			keywords:   []string{"adobe"},
		},
		{
			// adobe-client-secret
			expression: `(p8e-)(?i)[a-z0-9]{32}`,
			keywords:   []string{"p8e-"},
		},
		{
			// alibaba-access-key-id
			expression: `([^0-9A-Za-z]|^)(?P<secret>(LTAI)(?i)[a-z0-9]{20})([^0-9A-Za-z]|$)`,
			keywords:   []string{"LTAI"},
		},
		{
			// alibaba-secret-key
			expression: `(?i)(?P<key>alibaba[a-z0-9_ .\-,]{0,25})(=|>|:=|\|\|:|<=|=>|:).{0,5}['\"](?P<secret>[a-z0-9]{30})['\"]`,
			keywords:   []string{"alibaba"},
		},
		{
			// asana-client-id
			expression: `(?i)(?P<key>asana[a-z0-9_ .\-,]{0,25})(=|>|:=|\|\|:|<=|=>|:).{0,5}['\"](?P<secret>[0-9]{16})['\"]`,
			keywords:   []string{"asana"},
		},
		{
			// asana-client-secret
			expression: `(?i)(?P<key>asana[a-z0-9_ .\-,]{0,25})(=|>|:=|\|\|:|<=|=>|:).{0,5}['\"](?P<secret>[a-z0-9]{32})['\"]`,
			keywords:   []string{"asana"},
		},
		{
			// atlassian-api-token
			expression: `(?i)(?P<key>atlassian[a-z0-9_ .\-,]{0,25})(=|>|:=|\|\|:|<=|=>|:).{0,5}['\"](?P<secret>[a-z0-9]{24})['\"]`,
			keywords:   []string{"atlassian"},
		},
		{
			// bitbucket-client-id
			expression: `(?i)(?P<key>bitbucket[a-z0-9_ .\-,]{0,25})(=|>|:=|\|\|:|<=|=>|:).{0,5}['\"](?P<secret>[a-z0-9]{32})['\"]`,
			keywords:   []string{"bitbucket"},
		},
		{
			// bitbucket-client-secret
			expression: `(?i)(?P<key>bitbucket[a-z0-9_ .\-,]{0,25})(=|>|:=|\|\|:|<=|=>|:).{0,5}['\"](?P<secret>[a-z0-9_\-]{64})['\"]`,
			keywords:   []string{"bitbucket"},
		},
		{
			// beamer-api-token
			expression: `(?i)(?P<key>beamer[a-z0-9_ .\-,]{0,25})(=|>|:=|\|\|:|<=|=>|:).{0,5}['\"](?P<secret>b_[a-z0-9=_\-]{44})['\"]`,
			keywords:   []string{"beamer"},
		},
		{
			// clojars-api-token
			expression: `(CLOJARS_)(?i)[a-z0-9]{60}`,
			keywords:   []string{"CLOJARS_"},
		},
		{
			// contentful-delivery-api-token
			expression: `(?i)(?P<key>contentful[a-z0-9_ .\-,]{0,25})(=|>|:=|\|\|:|<=|=>|:).{0,5}['\"](?P<secret>[a-z0-9\-=_]{43})['\"]`,
			keywords:   []string{"contentful"},
		},
		{
			// databricks-api-token
			expression: `dapi[a-h0-9]{32}`,
			keywords:   []string{"dapi"},
		},
		{
			// discord-api-token
			expression: `(?i)(?P<key>discord[a-z0-9_ .\-,]{0,25})(=|>|:=|\|\|:|<=|=>|:).{0,5}['\"](?P<secret>[a-h0-9]{64})['\"]`,
			keywords:   []string{"discord"},
		},
		{
			// discord-client-id
			expression: `(?i)(?P<key>discord[a-z0-9_ .\-,]{0,25})(=|>|:=|\|\|:|<=|=>|:).{0,5}['\"](?P<secret>[0-9]{18})['\"]`,
			keywords:   []string{"discord"},
		},
		{
			// discord-client-secret
			expression: `(?i)(?P<key>discord[a-z0-9_ .\-,]{0,25})(=|>|:=|\|\|:|<=|=>|:).{0,5}['\"](?P<secret>[a-z0-9=_\-]{32})['\"]`,
			keywords:   []string{"discord"},
		},
		{
			// doppler-api-token
			expression: `['\"](dp\.pt\.)(?i)[a-z0-9]{43}['\"]`,
			keywords:   []string{"dp.pt."},
		},
		{
			// dropbox-api-secret
			expression: `(?i)(dropbox[a-z0-9_ .\-,]{0,25})(=|>|:=|\|\|:|<=|=>|:).{0,5}['\"]([a-z0-9]{15})['\"]`,
			keywords:   []string{"dropbox"},
		},
		{
			// dropbox-short-lived-api-token
			expression: `(?i)(dropbox[a-z0-9_ .\-,]{0,25})(=|>|:=|\|\|:|<=|=>|:).{0,5}['\"](sl\.[a-z0-9\-=_]{135})['\"]`,
			keywords:   []string{"dropbox"},
		},
		{
			// dropbox-long-lived-api-token
			expression: `(?i)(dropbox[a-z0-9_ .\-,]{0,25})(=|>|:=|\|\|:|<=|=>|:).{0,5}['\"][a-z0-9]{11}(AAAAAAAAAA)[a-z0-9\-_=]{43}['\"]`,
			keywords:   []string{"dropbox"},
		},
		{
			// duffel-api-token
			expression: `['\"]duffel_(test|live)_(?i)[a-z0-9_-]{43}['\"]`,
			keywords:   []string{"duffel_test_", "duffel_live_"},
		},
		{
			// dynatrace-api-token
			expression: `['\"]dt0c01\.(?i)[a-z0-9]{24}\.[a-z0-9]{64}['\"]`,
			keywords:   []string{"dt0c01."},
		},
		{
			// easypost-api-token
			expression: `['\"]EZ[AT]K(?i)[a-z0-9]{54}['\"]`,
			keywords:   []string{"EZAK", "EZAT"},
		},
		{
			// fastly-api-token
			expression: `(?i)(?P<key>fastly[a-z0-9_ .\-,]{0,25})(=|>|:=|\|\|:|<=|=>|:).{0,5}['\"](?P<secret>[a-z0-9\-=_]{32})['\"]`,
			keywords:   []string{"fastly"},
		},
		{
			// finicity-client-secret
			expression: `(?i)(?P<key>finicity[a-z0-9_ .\-,]{0,25})(=|>|:=|\|\|:|<=|=>|:).{0,5}['\"](?P<secret>[a-z0-9]{20})['\"]`,
			keywords:   []string{"finicity"},
		},
		{
			// finicity-api-token
			expression: `(?i)(?P<key>finicity[a-z0-9_ .\-,]{0,25})(=|>|:=|\|\|:|<=|=>|:).{0,5}['\"](?P<secret>[a-f0-9]{32})['\"]`,
			keywords:   []string{"finicity"},
		},
		{
			// flutterwave-public-key
			expression: withoutWordPrefix(`?P<secret>FLW(PUB|SEC)K_TEST-(?i)[a-h0-9]{32}-X`),
			keywords:   []string{"FLWSECK_TEST-", "FLWPUBK_TEST-"},
		},
		{
			// flutterwave-enc-key
			expression: withoutWordPrefix(`?P<secret>FLWSECK_TEST[a-h0-9]{12}`),
			keywords:   []string{"FLWSECK_TEST"},
		},
		{
			// frameio-api-token
			expression: `fio-u-(?i)[a-z0-9\-_=]{64}`,
			keywords:   []string{"fio-u-"},
		},
		{
			// gocardless-api-token
			expression: `['\"]live_(?i)[a-z0-9\-_=]{40}['\"]`,
			keywords:   []string{"live_"},
		},
		{
			// grafana-api-token
			expression: `['\"]?eyJrIjoi(?i)[a-z0-9\-_=]{72,92}['\"]?`,
			keywords:   []string{"eyJrIjoi"},
		},
		{
			// hashicorp-tf-api-token
			expression: `['\"](?i)[a-z0-9]{14}\.atlasv1\.[a-z0-9\-_=]{60,70}['\"]`,
			keywords:   []string{"atlasv1."},
		},
		{
			// hubspot-api-token
			expression: `(?i)(?P<key>hubspot[a-z0-9_ .\-,]{0,25})(=|>|:=|\|\|:|<=|=>|:).{0,5}['\"](?P<secret>[a-h0-9]{8}-[a-h0-9]{4}-[a-h0-9]{4}-[a-h0-9]{4}-[a-h0-9]{12})['\"]`,
			keywords:   []string{"hubspot"},
		},
		{
			// intercom-api-token
			expression: `(?i)(?P<key>intercom[a-z0-9_ .\-,]{0,25})(=|>|:=|\|\|:|<=|=>|:).{0,5}['\"](?P<secret>[a-z0-9=_]{60})['\"]`,
			keywords:   []string{"intercom"},
		},
		{
			// intercom-client-secret
			expression: `(?i)(?P<key>intercom[a-z0-9_ .\-,]{0,25})(=|>|:=|\|\|:|<=|=>|:).{0,5}['\"](?P<secret>[a-h0-9]{8}-[a-h0-9]{4}-[a-h0-9]{4}-[a-h0-9]{4}-[a-h0-9]{12})['\"]`,
			keywords:   []string{"intercom"},
		},
		{
			// ionic-api-token
			expression: `(?i)(ionic[a-z0-9_ .\-,]{0,25})(=|>|:=|\|\|:|<=|=>|:).{0,5}['\"](ion_[a-z0-9]{42})['\"]`,
			keywords:   []string{"ionic"},
		},
		{
			// jwt-token
			expression: `ey[a-zA-Z0-9]{17,}\.ey[a-zA-Z0-9\/\\_-]{17,}\.(?:[a-zA-Z0-9\/\\_-]{10,}={0,2})?`,
			keywords:   []string{".eyJ"},
		},
		{
			// linear-api-token
			expression: `lin_api_(?i)[a-z0-9]{40}`,
			keywords:   []string{"lin_api_"},
		},
		{
			// linear-client-secret
			expression: `(?i)(?P<key>linear[a-z0-9_ .\-,]{0,25})(=|>|:=|\|\|:|<=|=>|:).{0,5}['\"](?P<secret>[a-f0-9]{32})['\"]`,
			keywords:   []string{"linear"},
		},
		{
			// lob-api-key
			expression: `(?i)(?P<key>lob[a-z0-9_ .\-,]{0,25})(=|>|:=|\|\|:|<=|=>|:).{0,5}['\"](?P<secret>(live|test)_[a-f0-9]{35})['\"]`,
			keywords:   []string{"lob"},
		},
		{
			// lob-pub-api-key
			expression: `(?i)(?P<key>lob[a-z0-9_ .\-,]{0,25})(=|>|:=|\|\|:|<=|=>|:).{0,5}['\"](?P<secret>(test|live)_pub_[a-f0-9]{31})['\"]`,
			keywords:   []string{"lob"},
		},
		{
			// mailchimp-api-key
			expression: `(?i)(?P<key>mailchimp[a-z0-9_ .\-,]{0,25})(=|>|:=|\|\|:|<=|=>|:).{0,5}['\"](?P<secret>[a-f0-9]{32}-us20)['\"]`,
			keywords:   []string{"mailchimp"},
		},
		{
			// mailgun-token
			expression: `(?i)(?P<key>mailgun[a-z0-9_ .\-,]{0,25})(=|>|:=|\|\|:|<=|=>|:).{0,5}['\"](?P<secret>(pub)?key-[a-f0-9]{32})['\"]`,
			keywords:   []string{"mailgun"},
		},
		{
			// mailgun-signing-key
			expression: `(?i)(?P<key>mailgun[a-z0-9_ .\-,]{0,25})(=|>|:=|\|\|:|<=|=>|:).{0,5}['\"](?P<secret>[a-h0-9]{32}-[a-h0-9]{8}-[a-h0-9]{8})['\"]`,
			keywords:   []string{"mailgun"},
		},
		{
			// mapbox-api-token
			expression: `(?i)(pk\.[a-z0-9]{60}\.[a-z0-9]{22})`,
			keywords:   []string{"pk."},
		},
		{
			// messagebird-api-token
			expression: `(?i)(?P<key>messagebird[a-z0-9_ .\-,]{0,25})(=|>|:=|\|\|:|<=|=>|:).{0,5}['\"](?P<secret>[a-z0-9]{25})['\"]`,
			keywords:   []string{"messagebird"},
		},
		{
			// messagebird-client-id
			expression: `(?i)(?P<key>messagebird[a-z0-9_ .\-,]{0,25})(=|>|:=|\|\|:|<=|=>|:).{0,5}['\"](?P<secret>[a-h0-9]{8}-[a-h0-9]{4}-[a-h0-9]{4}-[a-h0-9]{4}-[a-h0-9]{12})['\"]`,
			keywords:   []string{"messagebird"},
		},
		{
			// new-relic-user-api-key
			expression: `['\"](NRAK-[A-Z0-9]{27})['\"]`,
			keywords:   []string{"NRAK-"},
		},
		{
			// new-relic-user-api-id
			expression: `(?i)(?P<key>newrelic[a-z0-9_ .\-,]{0,25})(=|>|:=|\|\|:|<=|=>|:).{0,5}['\"](?P<secret>[A-Z0-9]{64})['\"]`,
			keywords:   []string{"newrelic"},
		},
		{
			// new-relic-browser-api-token
			expression: `['\"](NRJS-[a-f0-9]{19})['\"]`,
			keywords:   []string{"NRJS-"},
		},
		{
			// npm-access-token
			expression: `['\"](npm_(?i)[a-z0-9]{36})['\"]`,
			keywords:   []string{"npm_"},
		},
		{
			// planetscale-password
			expression: `pscale_pw_(?i)[a-z0-9\-_\.]{43}`,
			keywords:   []string{"pscale_pw_"},
		},
		{
			// planetscale-api-token
			expression: `pscale_tkn_(?i)[a-z0-9\-_\.]{43}`,
			keywords:   []string{"pscale_tkn_"},
		},
		{
			// private-packagist-token
			expression: `packagist_[ou][ru]t_(?i)[a-f0-9]{68}`,
			keywords:   []string{"packagist_uut_", "packagist_ort_", "packagist_out_"},
		},
		{
			// postman-api-token
			expression: `PMAK-(?i)[a-f0-9]{24}\-[a-f0-9]{34}`,
			keywords:   []string{"PMAK-"},
		},
		{
			// pulumi-api-token
			expression: `pul-[a-f0-9]{40}`,
			keywords:   []string{"pul-"},
		},
		{
			// rubygems-api-token
			expression: `rubygems_[a-f0-9]{48}`,
			keywords:   []string{"rubygems_"},
		},
		{
			// sendgrid-api-token
			expression: `SG\.(?i)[a-z0-9_\-\.]{66}`,
			keywords:   []string{"SG."},
		},
		{
			// sendinblue-api-token
			expression: `xkeysib-[a-f0-9]{64}\-(?i)[a-z0-9]{16}`,
			keywords:   []string{"xkeysib-"},
		},
		{
			// shippo-api-token
			expression: `shippo_(live|test)_[a-f0-9]{40}`,
			keywords:   []string{"shippo_live_", "shippo_test_"},
		},
		{
			// linkedin-client-secret
			expression: `(?i)(?P<key>linkedin[a-z0-9_ .\-,]{0,25})(=|>|:=|\|\|:|<=|=>|:).{0,5}['\"](?P<secret>[a-z]{16})['\"]`,
			keywords:   []string{"linkedin"},
		},
		{
			// linkedin-client-id
			expression: `(?i)(?P<key>linkedin[a-z0-9_ .\-,]{0,25})(=|>|:=|\|\|:|<=|=>|:).{0,5}['\"](?P<secret>[a-z0-9]{14})['\"]`,
			keywords:   []string{"linkedin"},
		},
		{
			// twitch-api-token
			expression: `(?i)(?P<key>twitch[a-z0-9_ .\-,]{0,25})(=|>|:=|\|\|:|<=|=>|:).{0,5}['\"](?P<secret>[a-z0-9]{30})['\"]`,
			keywords:   []string{"twitch"},
		},
		{
			// typeform-api-token
			expression: `(?i)(?P<key>typeform[a-z0-9_ .\-,]{0,25})(=|>|:=|\|\|:|<=|=>|:).{0,5}(?P<secret>tfp_[a-z0-9\-_\.=]{59})`,
			keywords:   []string{"typeform"},
		},
		{
			// dockerconfig-secret
			expression: `(?i)(\.(dockerconfigjson|dockercfg):\s*\|*\s*(?P<secret>(ey|ew)+[A-Za-z0-9\/\+=]+))`,
			keywords:   []string{"dockerc"},
		},
		{
			// docker pat
			expression: `(?i)(dckr_pat_[-0-9a-zA-Z]{27})`,
			keywords:   []string{"dckr_pat"},
		},
	}
})
