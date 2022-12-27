# go-stocks

### This is a go project with real-time processing pipeline using go concurrency to check latest price data of a ticker from yahoo finance, and if there's a 5% increase/decrease will send mail to specified mail through SMTP.

<br />

create <code>creds.env</code> 
```
EMAIL=<your gmail>
PASSWORD=<gmail app password>
```
To get App password, 
go to Google account settings -> Security -> Select app -> "Other (Custom name)" and enter a name for the app (e.g. "Golang SMTP").

do 
```
go get -u github.com/joho/godotenv
go get -u github.com/shopspring/decimal
go get -u github.com/tidwall/gjson
```
then
```
go run main.go
```