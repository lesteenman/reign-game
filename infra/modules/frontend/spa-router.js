// CloudFront Function (viewer-request) attached to the default S3 behavior only.
// Rewrites SPA navigation routes to /index.html so client-side routing works,
// while leaving static assets (so missing files return a real 404) and /api/*
// (a separate behavior) untouched. This replaces distribution-wide
// custom_error_response 403/404 -> /index.html, which also rewrote /api/*
// error responses and masked real API status codes.
function handler(event) {
  var request = event.request;
  var uri = request.uri;

  // API requests live on a separate cache behavior and must keep their real
  // status codes; never rewrite them (defensive — this function is not
  // associated with the /api/* behavior).
  if (uri.startsWith('/api/')) {
    return request;
  }

  // Static assets: the last path segment has a file extension. Pass through so
  // a missing file returns a real 404 instead of the SPA shell.
  var lastSegment = uri.substring(uri.lastIndexOf('/') + 1);
  if (lastSegment.indexOf('.') !== -1) {
    return request;
  }

  // SPA navigation route — serve the app shell; client-side router takes over.
  request.uri = '/index.html';
  return request;
}
