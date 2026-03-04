# Servers Skipped for API Keys in results.json but Listed Tools in results.prior.json

32 servers were skipped in `results.json` due to API key/credential requirements, but previously had `status: "tools_found"` in `results.prior.json`.

The prior run started these servers and successfully called `list_tools` even without valid credentials. The current run skips them pre-emptively based on detected API key requirements.

| Server | Skip Reason | Prior Tool Count |
|---|---|---|
| `com.stackhawk/stackhawk@1.1.1` | STACKHAWK_API_KEY | 16 |
| `io.github.4R9UN/mcp-kql-server@2.0.8` | Azure Data Explorer credentials | 2 |
| `io.github.CodeLogicIncEngineering/codelogic-mcp-server@1.0.11` | CodeLogic SaaS credentials | 2 |
| `io.github.Memphora/memphora@0.1.2` | MEMPHORA_API_KEY | 5 |
| `io.github.OtherVibes/mcp-as-a-judge@0.3.20` | OpenAI/Anthropic API key | 8 |
| `io.github.PagerDuty/pagerduty-mcp@0.2.1` | PAGERDUTY_USER_API_KEY | 37 |
| `io.github.SamMorrowDrums/remarkable@0.6.0` | REMARKABLE_TOKEN | 6 |
| `io.github.YimingYAN/fivetran-mcp@0.3.1` | FIVETRAN_API_KEY + SECRET | 15 |
| `io.github.alondmnt/joplin-mcp@0.5.0` | JOPLIN_TOKEN | 19 |
| `io.github.clarenceh/expenselm-mcp-server@0.2.3` | EXPENSELM_API_KEY | 8 |
| `io.github.detailobsessed/unblu@0.4.3` | UNBLU_API_KEY | 5 |
| `io.github.galaxyproject/galaxy-mcp@1.3.0` | GALAXY_API_KEY | 31 |
| `io.github.henilcalagiya/google-sheets-mcp@0.1.6` | Google Sheets API credentials | 24 |
| `io.github.jackfioru92/aruba-email@0.2.1` | Aruba email credentials | 14 |
| `io.github.khglynn/spotify-bulk-actions-mcp@0.1.1` | SPOTIFY_CLIENT_ID/SECRET | 32 |
| `io.github.leshchenko1979/fast-mcp-telegram@0.4.5` | Telegram API_ID + API_HASH | 9 |
| `io.github.palewire/datawrapper-mcp@0.0.19` | DATAWRAPPER_ACCESS_TOKEN | 8 |
| `io.github.pree-dew/mcp-bookmark@0.1.1` | OPENAI_API_KEY | 2 |
| `io.github.qwe4559999/scopus-mcp@0.1.3` | SCOPUS_API_KEY | 3 |
| `io.github.saucelabs-sample-test-frameworks/sauce-api-mcp@1.0.0` | SAUCE_USERNAME + ACCESS_KEY | 30 |
| `io.github.scrape-badger/scrapebadger@0.1.1` | SCRAPEBADGER_API_KEY | 16 |
| `io.github.sidart10/runway-mcp-server@0.1.0` | RUNWAY_API_KEY | 12 |
| `io.github.signnow/sn-api-helper-mcp@0.1.1` | SignNow API credentials | 1 |
| `io.github.trickyfalcon/mcp-defender@0.1.0` | Microsoft Defender / Azure AD credentials | 2 |
| `io.github.trickyfalcon/mcp-msdefenderkql@1.0.0` | Microsoft Defender API credentials | 2 |
| `io.github.universalamateur/reclaim-mcp-server@0.9.1` | Reclaim.ai API key | 40 |
| `io.github.vemonet/openroute-mcp@0.0.4` | OPENROUTESERVICE_API_KEY | 6 |
| `io.github.verygoodplugins/robinhood-mcp@0.1.0` | Robinhood credentials | 12 |
| `io.github.wmarceau/apollo@1.1.1` | APOLLO_API_KEY | 9 |
| `io.github.wmarceau/rideshare-comparison@1.0.0` | RIDESHARE_API_KEY | 3 |
| `io.github.wmarceau/upwork@1.0.1` | Upwork API credentials | 9 |
| `io.github.wmarceau/youtube-creator@1.0.0` | Google Cloud OAuth | 11 |
