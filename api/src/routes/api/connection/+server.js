import { ERRORS } from '$lib/errors';

export function GET() {
    return ERRORS.wsRequired();
}
