This is what came out when I asked ChatGTP the following:

"Please write a minimal high-performace webserver, that answers all requests with status 200 and body "OK". 
 This webserver needs to be able to handle many requests (mainly GET, POST, about 4000-5000 per second) and run on linux.
 Please chose a fast and reliable programming language that is well made for low ressource consumption and high performance.
 Ideally the webserver regularly outputs some statistics about how many requests/s und the average request size (in bytes) were handled.
 This webserver is meant to be a destination for debugging request mirroring of haproxy with spoa-mirror."
