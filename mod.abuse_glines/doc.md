
I want to write a web application written in vue.js + vite that will do the following:

# Config options
network = Undernet
glinelookup_url = http://127.0.0.1:2000/api2/glinelookup/:network/:ip
gline_lookup_url = http://127.0.0.1:2000/api2/ismyipgline/:network


# First page (SPA)
* Editbox (containing the user's own ip address if possible) for which the associated label is `IP address:`
* Button `Lookup` on the right of the editbox
* When the button is clicked (or enter key is pressed in the editbox), make an API call to glinelookup_url, replacing :network with the network variable and :ip with the ip written in the editbox.

* If the API returns an empty list, show the following message on the first page: `IP address <ip> is not G-lined on <network>`

# Second page (SPA)
* The API should return a list of glines in json format. Example:
    ```json
    [
    {
        "active": true,
        "mask": "*@154.205.134.194",
        "expirets": 1736036121,
        "lastmodts": 1734826521,
        "hoursuntilexpire": 313,
        "reason": "AUTO [0] DNSBL listed. Check https://ircbl.org/lookup?ip=154.205.134.194\u0026network=undernet for removal."
    }
    ]
    ```
* If the reason contains `\u0026`, make sure to replace it with `&` instead.
* For each gline in the list, display the gline information to the user in a proper way. Display the following infos:
  * Gline mask
  * Calculate when the gline expires (expirets) and print it in a friendly duration format
    * Examples:
      * Expires in 2 days, 12 hours and 15 minutes. Do not write the seconds.
      * Expired in 2 days, 12 hours and 15 minutes. Do not write the seconds.
    * Also display the full date of the expire time (expirets), in the user's timezone, including UTC offset at the end.
  * Gline reason







# glinelookup api details
* The api will be called with the Authorization Bearer header. The key is stored in the the api_key config variable.
* Use lowercase(network) for the api call.
* curl command example:
    ```bash
    curl -X GET http://localhost:2000/api2/glinelookup/undernet/154.205.134.194 -H "Content-Type: application/json"
    ```
* The API should normally return 200 if everything goes fine. Otherwise, it failed. If it failed, show this message: `The gline lookup API call failed. Please email abuse@undernet.org to inform them about your G-line problem with <ip>`




# SPA - Part two (tasks)
* Write a function GetTasks() that takes uuid as param and calls the tasks api at /api/tasks/uuid (replace uuid with the uuid passed as parameter). The result will be returned as json.
* If the task_id is the same as one previously stored,
  * If TaskType == "confirmemail", then
    * emailConfirmed.value = task.data

## Tasks api data returned as json
```go
type TaskStruct struct {
	TaskID            string      `json:"task_id"`
	UUID              string      `json:"uuid"`
	TaskType          string      `json:"task_type"`
	Progress          int64       `json:"progress"`
	CreationTS        int64       `json:"creation_ts"`
	StartedTS         int64       `json:"started_ts"`
	LastUpdatedTS     int64       `json:"last_updated_ts"`
	CompletedTS       int64       `json:"completed_ts"`
	Message           string      `json:"message"`
	DataVisibleToUser string      `json:"data"` // email goes here when TaskType is confirmemail
}
```
