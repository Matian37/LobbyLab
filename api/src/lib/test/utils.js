import { expect } from 'vitest';

export async function expectError(response, errorFactory) {
    const err = errorFactory();
    expect(response.status).toBe(err.status);
    expect(await response.json()).toEqual(await err.json());
}
