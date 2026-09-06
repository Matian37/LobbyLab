import { vi } from 'vitest';

/**
 * Builds a SvelteKit `enhance`-style mock for form actions. It installs a
 * `submit` handler on the given node that intercepts the event, invokes the
 * provided `submitCallback` with a fake `SubmitFunction` argument object, and
 * then feeds a default result (produced by `defaultResultFn`) into the returned
 * `SubmitFunctionResult` handler.
 *
 * @param {() => unknown} defaultResultFn Produces the `result` value passed to
 *     the submit result handler.
 * @returns {(node: HTMLElement, submitCallback?: Function) => { destroy: () => void }}
 *     The `enhance` mock.
 */
export function enhanceMock(defaultResultFn) {
    return (node, submitCallback) => {
        const handleSubmit = async (event) => {
            event.preventDefault();

            if (!submitCallback) return;

            const resultHandler = await submitCallback({
                action: node.action,
                controller: new AbortController(),
                formElement: node,
                formData: new FormData(node),
                submitter: event.submitter || null,
                cancel: vi.fn(),
            });

            const defaultResult = defaultResultFn();

            if (typeof resultHandler === 'function') {
                await resultHandler({
                    result: defaultResult,
                    update: vi.fn(),
                });
            }
        };

        node.addEventListener('submit', handleSubmit);

        return {
            destroy: () => node.removeEventListener('submit', handleSubmit),
        };
    };
}
