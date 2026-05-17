// Re-export of ApiError from the api client so consumers outside
// services/ can do `instanceof` discrimination without importing the
// service module itself (which is forbidden from screens / leaf
// components per the architecture rule).
export { ApiError } from '../services/api';
