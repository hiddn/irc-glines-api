# TODO
* [ ] Add TestHandleGline280() to main_test.go
* [ ] Add support for dalnet
* [ ] Take care of the TODO written around line 301 in main.go:
  * //TODO: send "GLINE <mask>" to server, as it is impossible from the message to know from this message if the gline is active or not. The expiration time will be in the future, even if the gline is being deactivated. I have to make sure that I also adapt handeGline280() to be able to update the info instead of just insert.
* [ ] Maybe protect the API with a key
* [ ] Use API in
  * [ ] ircbl
  * [ ] undernet rbl
  * [ ] undernet web site
* [ ] 