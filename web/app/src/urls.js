let url = new URL(window.location.href);

// When the web interface is running on a different port than the API, for
// example when using the Vite devserver, setting the Vite --mode flag to
// "development" will force port 8080 for the API.
if (import.meta.env.MODE == 'development') {
  url.port = '8080';
}

const URLs = {
  api: `${url.protocol}//${url.hostname}:${url.port}/`,
  ws: `${url.protocol.replace('http', 'ws')}//${url.hostname}:${url.port}/`,
};

// console.log("Flamenco API:", URLs.api);
// console.log("Websocket   :", URLs.ws);

export function ws() {
  return URLs.ws;
}
export function api() {
  return URLs.api;
}

// Backend URLs (like task logs, SwaggerUI, etc.) should be relative to the API
// url in order to stay working when the web development server is in use.
export function backendURL(path) {
  const url = new URL(path, URLs.api);
  return url.href;
}
