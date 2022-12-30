# TODO
* [ ] Restrict output in both API and irc. With the recent commit 5 mins ago, it is now possible to lookup by network cidr.
* [ ] Use integrated privmsgf function instead of my own, which will split messages if it exceeds a certain amount of chars
* [ ] Add TestHandleGline280() to main_test.go
* [ ] Add support for dalnet
* [ ] Take care of the TODO written around line 301 in main.go:
  * //TODO: send "GLINE <mask>" to server, as it is impossible from the message to know from this message if the gline is active or not. The expiration time will be in the future, even if the gline is being deactivated. I have to make sure that I also adapt handeGline280() to be able to update the info instead of just insert.
* [ ] Maybe protect the API with a key
* [ ] Use API in
  * [ ] ircbl
  * [ ] undernet rbl
  * [ ] undernet web site
* [ ] Make listen port and interface configurable for the API
* [ ] Modify cidranger to add a new function that allows to check intersection between two cidr ranges.

# Comments from Ratler
* [ ] make use of c.Bind() instead of c.Param(), and bind the request input to a struct, much cleaner.
  * Allows to easily plugin validation on the params
* [ ] add two middleware to echo. Logging() and Recover()
  * That way you can use c.Logger().Info("bla bla"). And you control the behavior globally, like logformat etc.
  * Recover() is great if you get a panic, because it will properly capture it and throw good trace instead of just crashing
* [ ] Make the app a package
