import { it, expect, describe } from 'vitest';
import { formatTime, formatButton, matchStatusString } from './format.js';
import { WaitingStatus } from './constants.js';

describe('formatTime', () => {
    it.each([
        [0, '00:00'],
        [59, '00:59'],
        [60, '01:00'],
        [3599, '59:59'],
        [3600, '01:00:00'],
        [3661, '01:01:01'],
        [7325, '02:02:05'],
        [360000, '100:00:00'],
    ])('formats %i seconds as %s', (seconds, expected) => {
        expect(formatTime(seconds)).toBe(expected);
    });
});

describe('formatButton', () => {
    it('returns Play when not active', () => {
        expect(formatButton(WaitingStatus.NOT_ACTIVE, 65)).toBe('Play');
    });

    it('returns Cancel with the elapsed time when pending', () => {
        expect(formatButton(WaitingStatus.PENDING, 65)).toBe('Cancel 01:05');
    });

    it('returns Join when a match is found', () => {
        expect(formatButton(WaitingStatus.FOUND, 65)).toBe('Join');
    });
});

describe('matchStatusString', () => {
    it.each([
        [{ active: true, canceled: false }, 'ACTIVE'],
        [{ active: true, canceled: true }, 'ACTIVE'],
        [{ active: false, canceled: true }, 'CANCELED'],
        [{ active: false, canceled: false }, 'FINISHED'],
    ])('returns %s for %o', (match, expected) => {
        expect(matchStatusString(match)).toBe(expected);
    });
});
