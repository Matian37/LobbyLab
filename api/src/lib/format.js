import { WaitingStatus } from '$lib/constants.js';

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

export function matchStatusString(match) {
    if (match.active) return 'ACTIVE';
    if (match.canceled) return 'CANCELED';
    return 'FINISHED';
}
