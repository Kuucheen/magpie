import { AbstractControl, ValidationErrors, ValidatorFn, Validators } from '@angular/forms';

export const passwordMinLength = 12;
const uppercasePattern = /[A-Z]/;
const lowercasePattern = /[a-z]/;
const digitPattern = /\d/;
const whitespacePattern = /\s/;

export function passwordPolicyValidators(): ValidatorFn[] {
  return [
    Validators.required,
    Validators.minLength(passwordMinLength),
    passwordPolicyValidator(),
  ];
}

export function passwordPolicyValidator(): ValidatorFn {
  return (control: AbstractControl): ValidationErrors | null => {
    const rawValue = control.value;
    if (typeof rawValue !== 'string' || rawValue.length === 0) {
      return null;
    }

    const errors: ValidationErrors = {};

    if (!lowercasePattern.test(rawValue)) {
      errors['missingLowercase'] = true;
    }
    if (!uppercasePattern.test(rawValue)) {
      errors['missingUppercase'] = true;
    }
    if (!digitPattern.test(rawValue)) {
      errors['missingDigit'] = true;
    }
    if (whitespacePattern.test(rawValue)) {
      errors['containsWhitespace'] = true;
    }

    return Object.keys(errors).length > 0 ? errors : null;
  };
}

export function passwordPolicyMessages(): string[] {
  return [
    `Use at least ${passwordMinLength} characters.`,
    'Include at least one uppercase letter.',
    'Include at least one lowercase letter.',
    'Include at least one number.',
    'Do not use spaces.',
  ];
}
