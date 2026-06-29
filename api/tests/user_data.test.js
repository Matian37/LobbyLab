import { vi, test, expect, describe, beforeEach } from 'vitest';
import * as data from '$lib/user_data.js';

test.beforeEach(async () => {
    localStorage.clear();
});

vi.stubGlobal('fetch', vi.fn().mockResolvedValue({
    ok: true,
    json: async () => ({ sukces: true })
}));

test('Loading empty user', async () => {
    expect(await data.getData()).toBe(false);
});

test('Loading user with storage', async() => {
   localStorage.setItem('logged', JSON.stringify({token: "123456789", login: "user"}));
   expect(await data.getData()).toEqual({token: "123456789", login: "user"});
});

test('loading user with storage but with expired session', async () => {
    fetch.mockResolvedValueOnce({
        ok: true,
        json: async () => ({ sukces: false })
    });
    localStorage.setItem('logged', JSON.stringify({token: "123456789", login: "user"}));

    expect(await data.getData()).toBe(false);
});

test('update data and loading it', async () => {
    expect(await data.getData()).toBe(false);
    await data.updateData({token: "1234567890qwerty", login: "user"});
    expect(await data.getData()).toEqual({token: "1234567890qwerty", login: "user"});
    data.resetData();
    expect(await data.getData()).toBe(false);
});
