// Re-export of ApiError from the api client. Pre-#120 this existed
// because `@services/api` was forbidden from screens / leaf components
// per the architecture rule; now `@shared/api` lives in the shared
// layer and is already allowed from anywhere below `app/`. This
// alias is kept for one release as a soft-deprecation: existing call
// sites that import from `@shared/api-errors` still resolve. New code
// should `import { ApiError } from '@shared/api'` directly.
export { ApiError } from '@shared/api';
