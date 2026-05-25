import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { UIErrorHandler } from '../utils/error-handler.js';

describe('UIErrorHandler', () => {
    beforeEach(() => {
        // Setup DOM for showToast
        document.body.innerHTML = '<div id="import-result"></div>';
        vi.spyOn(console, 'error').mockImplementation(() => {});
        vi.spyOn(window, 'alert').mockImplementation(() => {});
    });

    afterEach(() => {
        vi.restoreAllMocks();
    });

    describe('notify', () => {
        it('should redirect to login on 401 error', () => {
            // Mock window.location.href
            const origLocation = window.location;
            delete window.location;
            window.location = { href: '' };

            UIErrorHandler.notify(new Error('401 Unauthorized'));
            expect(window.location.href).toBe('../index.html');

            window.location = origLocation;
        });

        it('should show detailed modal for 409 conflict', () => {
            const spy = vi.spyOn(UIErrorHandler, 'showDetailedModal');
            UIErrorHandler.notify(new Error('409 Conflict'));
            expect(spy).toHaveBeenCalledWith('資料衝突', '409 Conflict');
        });

        it('should show toast for general errors', () => {
            UIErrorHandler.notify(new Error('Something went wrong'));
            const feedback = document.getElementById('import-result');
            expect(feedback.textContent).toContain('Something went wrong');
            expect(feedback.style.color).toBe('rgb(231, 76, 60)');
        });
    });

    describe('showToast', () => {
        it('should set message on import-result element', () => {
            UIErrorHandler.showToast('Test message');
            const el = document.getElementById('import-result');
            expect(el.textContent).toBe('Test message');
            expect(el.style.color).toBe('rgb(231, 76, 60)');
        });

        it('should use body fallback when import-result missing', () => {
            document.body.innerHTML = '';
            UIErrorHandler.showToast('Fallback');
            expect(document.body.textContent).toBe('Fallback');
        });
    });

    describe('showDetailedModal', () => {
        it('should create a modal with title and message', () => {
            UIErrorHandler.showDetailedModal('Error Title', 'Error details here');
            const modal = document.querySelector('.error-detail-modal');
            expect(modal).not.toBeNull();
            expect(modal.innerHTML).toContain('Error Title');
            expect(modal.innerHTML).toContain('Error details here');
        });

        it('should remove existing modal before creating new one', () => {
            UIErrorHandler.showDetailedModal('First', 'First message');
            UIErrorHandler.showDetailedModal('Second', 'Second message');
            const modals = document.querySelectorAll('.error-detail-modal');
            expect(modals.length).toBe(1);
            expect(modals[0].innerHTML).toContain('Second');
        });
    });

    describe('handleConflict', () => {
        it('should call showDetailedModal with conflict message', () => {
            const spy = vi.spyOn(UIErrorHandler, 'showDetailedModal');
            UIErrorHandler.handleConflict({});
            expect(spy).toHaveBeenCalledWith('資料衝突', '發現資料衝突，請檢查後再試一次。');
        });
    });

    describe('handleAuthError', () => {
        it('should redirect to login', () => {
            const origLocation = window.location;
            delete window.location;
            window.location = { href: '' };

            UIErrorHandler.handleAuthError();
            expect(window.location.href).toBe('../index.html');

            window.location = origLocation;
        });
    });
});