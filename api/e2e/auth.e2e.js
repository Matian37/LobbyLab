import { test, expect } from './fixtures.js';
import { BASE_URL, registerViaUi } from './helpers.js';

test.describe('authentication', () => {
    test('registers a new user and lands on the home page logged in', async ({
        page,
    }) => {
        await page.goto(`${BASE_URL}/register`);
        await page.getByTestId('login-input').fill('login');
        await page.getByTestId('password-input').fill('password');
        await page.getByTestId('register-apply').click();
        await page.waitForURL(`${BASE_URL}/`);

        await expect(page.getByTestId('title')).toHaveText('login');
        await expect(page.getByTestId('download-client')).toBeVisible();
    });

    test('rejects a duplicate registration', async ({ page }) => {
        await registerViaUi(page, 'login');

        await page.goto(`${BASE_URL}/register`);
        await page.getByTestId('login-input').fill('login');
        await page.getByTestId('password-input').fill('password');
        await page.getByTestId('register-apply').click();

        await expect(page.getByTestId('error-text')).toHaveText(
            'Login is already taken'
        );
    });

    test('logs in with correct credentials', async ({ page }) => {
        await registerViaUi(page, 'login');

        await page.getByTestId('logout').click();
        await page.goto(`${BASE_URL}/login`);
        await page.getByTestId('login-input').fill('login');
        await page.getByTestId('password-input').fill('password');
        await page.getByTestId('login-apply').click();
        await page.waitForURL(`${BASE_URL}/`);

        await expect(page).toHaveURL(BASE_URL + '/');
        await expect(page.getByTestId('title')).toHaveText('login');
    });

    test('rejects login with wrong credentials', async ({ page }) => {
        await registerViaUi(page, 'login');

        await page.goto(`${BASE_URL}/login`);
        await page.getByTestId('login-input').fill('login');
        await page.getByTestId('password-input').fill('wrong');
        await page.getByTestId('login-apply').click();

        await expect(page.getByTestId('error-text')).toHaveText(
            'Invalid login or password'
        );
    });

    test('logs out via the UI and returns to the logged out view', async ({
        page,
    }) => {
        await registerViaUi(page, 'login');

        await page.getByTestId('logout').click();

        await expect(page.getByTestId('title')).toHaveText('Log in');
    });

    test('shows login and register when logged out, home controls when logged in', async ({
        page,
    }) => {
        await page.goto(BASE_URL + '/');

        await expect(page.getByTestId('login-page')).toBeVisible();
        await expect(page.getByTestId('register-page')).toBeVisible();
        await expect(page.getByTestId('logout-form')).not.toBeVisible();
        await expect(page.getByTestId('play')).not.toBeVisible();
        await expect(page.getByTestId('download-client')).not.toBeVisible();
        await expect(page.getByTestId('matches-title')).not.toBeVisible();

        await registerViaUi(page, 'login');

        await expect(page.getByTestId('login-page')).not.toBeVisible();
        await expect(page.getByTestId('register-page')).not.toBeVisible();
        await expect(page.getByTestId('logout-form')).toBeVisible();
        await expect(page.getByTestId('play')).toBeVisible();
        await expect(page.getByTestId('download-client')).toBeVisible();
        await expect(page.getByTestId('matches-title')).toBeVisible();
    });
});
