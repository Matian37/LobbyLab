import { vi } from 'vitest';

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
