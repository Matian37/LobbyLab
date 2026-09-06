import { WaitingStatus } from '$lib/constants.js';

/**
 * Formats a duration in seconds as `HH:MM:SS`, or `MM:SS` when it is under an
 * hour.
 *
 * @param {number} totalSeconds Duration in seconds. Values are floored and
 *     split into hours/minutes/seconds, each zero-padded to two digits.
 * @returns {string} The formatted duration.
 * @example
 * formatTime(0)      // '00:00'
 * formatTime(65)     // '01:05'
 * formatTime(3661)   // '01:01:01'
 */
export function formatTime(totalSeconds) {
    const hours = String(Math.floor(totalSeconds / 3600)).padStart(2, '0');
    const minutes = String(Math.floor((totalSeconds % 3600) / 60)).padStart(
        2,
        '0'
    );
    const seconds = String(totalSeconds % 60).padStart(2, '0');

    if (hours > 0) {
        return `${hours}:${minutes}:${seconds}`;
    }
    return `${minutes}:${seconds}`;
}

/**
 * Returns the matchmaking button label for a given waiting status. While
 * pending, the label includes how long the user has been queued.
 *
 * @param {keyof typeof WaitingStatus | WaitingStatus} status Current waiting status.
 * @param {number} totalSeconds Seconds already queued, shown when pending.
 * @returns {string} The button label (`'Play'`, `'Cancel …'`, or `'Join'`).
 */
export function formatButton(status, totalSeconds) {
    switch (status) {
        case WaitingStatus.NOT_ACTIVE:
            return 'Play';
        case WaitingStatus.PENDING:
            return 'Cancel ' + formatTime(totalSeconds);
        case WaitingStatus.FOUND:
            return 'Join';
    }
}

/**
 * Produces a human-readable status label for a match.
 *
 * @param {{ active: boolean, canceled: boolean }} match Match summary row.
 * @returns {string} `'ACTIVE'` when the match is running, `'CANCELED'` when it
 *     was canceled, otherwise `'FINISHED'`.
 */
export function matchStatusString(match) {
    if (match.active) return 'ACTIVE';
    if (match.canceled) return 'CANCELED';
    return 'FINISHED';
}
