import { vi, it, expect, describe, beforeEach, afterEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/svelte';
import { cleanup } from '@testing-library/svelte';
import userEvent from '@testing-library/user-event';
import '@testing-library/jest-dom/vitest';
import Page from './+page.svelte';
import { UNEXPECTED_ERROR_MSG } from '$lib/errors.js';
import { enhanceMock } from '$lib/test/forms.js';

const mocks = vi.hoisted(() => ({
    goto: vi.fn(),
    resolve: (path) => path,
    enhance: vi.fn(),
    nextResult: undefined,
}));

vi.mock('$app/navigation', () => ({ goto: mocks.goto }));
vi.mock('$app/paths', () => ({ resolve: mocks.resolve }));
vi.mock('$app/forms', () => ({ enhance: mocks.enhance }));

describe('register page', () => {
    beforeEach(() => {
        vi.clearAllMocks();
        mocks.nextResult = undefined;
        mocks.enhance.mockImplementation(enhanceMock(() => mocks.nextResult));
        render(Page);
    });

    afterEach(() => {
        cleanup();
        vi.unstubAllGlobals();
    });

    async function submit(login = 'login', password = 'password') {
        const user = userEvent.setup();
        await user.type(screen.getByTestId('login-input'), login);
        await user.type(screen.getByTestId('password-input'), password);
        await fireEvent.submit(screen.getByTestId('register-form'));
    }

    it('submits the form and navigates home on success', async () => {
        mocks.nextResult = { type: 'success', data: {} };

        await submit();

        await waitFor(() => expect(mocks.goto).toHaveBeenCalledWith('/'));
        expect(screen.getByTestId('error-text')).toHaveTextContent('');
    });

    it('shows the error when the login is already taken', async () => {
        mocks.nextResult = {
            type: 'failure',
            data: { msg: 'Login is already taken' },
        };

        await submit();

        await waitFor(() =>
            expect(screen.getByTestId('error-text')).toHaveTextContent(
                'Login is already taken'
            )
        );
        expect(mocks.goto).not.toHaveBeenCalled();
    });

    it('shows the unexpected error when the action errors', async () => {
        mocks.nextResult = { type: 'error', error: new Error('boom') };

        await submit();

        await waitFor(() =>
            expect(screen.getByTestId('error-text')).toHaveTextContent(
                UNEXPECTED_ERROR_MSG
            )
        );
        expect(mocks.goto).not.toHaveBeenCalled();
    });

    it('navigates back to the home page', async () => {
        const user = userEvent.setup();
        await user.click(screen.getByText('Back'));

        expect(mocks.goto).toHaveBeenCalledWith('/');
    });
});
