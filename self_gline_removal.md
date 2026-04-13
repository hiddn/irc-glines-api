# Abuse - self-removal gline web app for regex-based glines set by D

## How it works - regex gline based self-removal

#### Steps
1. User visits https://abuse.undernet.org (or glined.undernet.org or another name)
2. Web app detects if the user's ipv4 or ipv6 address is glined
3. If it is, it shows the gline message to the user and the user has to enter his email address
4. It allows the user to self-remove the gline if :
   * The gline message matches D's regex based glines
   * The user passes Recaptcha
   * The user confirms his email address or authenticates to X

<br/>

In Phase 2,
* Users could make their gline removal request via a form, even if their own ip isn't glined.
* Infos could be displayed to


#### Requirements
* [ ] Recaptcha
* [ ] Possibly X authentification
* [ ] Email confirmation (especially if not authed to X)
* [ ] ipv4 and ipv6 verification - user's ip must match an active regex-based gline


## Backend - golang
* [ ] Recaptcha validation API
* [ ] Send email - gopkg.in/gomail.v2 - https://pkg.go.dev/gopkg.in/gomail.v2#section-readme
  * [ ] Validations of email confirmation code
* [ ] Call ircbl's API to remove gline

## Frontend - vuejs


## Cloudflare
* [ ] Setup ipv4 exclusive A record that only accept ipv4
* [ ] Setup ipv6 exclusive AAAA record that only accept ipv6



## Future enhancements
* [ ] Request gline removal for any gline
* [ ] Oper-abuse complaint
  * [ ] gline based
  * [ ] other
* [ ] Other type of abuse reports
* [ ] PostgreSQL database integration to keep track and history

