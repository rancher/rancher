var http = require('http');
var url = require('url');
var number = 0;

http.createServer(function (req, res) {
    var urlParts = url.parse(req.url);
    if (urlParts.pathname !== '/favicon.ico') {
        if ( urlParts.pathname === '/metrics' ) {
            res.writeHead(200, { 'Content-Type': 'text/plain' }); 
            res.end('web_app_online_user_count ' + number + '\n'); 
        } else {
            number++;
            console.log('logan');
            res.writeHead(200, { 'Content-Type': 'text/plain' });
            res.end('Hello There!' + number + '\n'); 
        }
    }
}).listen(8080);
