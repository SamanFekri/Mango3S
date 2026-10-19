import { useState, type InputHTMLAttributes } from 'react'
import { EyeIcon, EyeOffIcon } from './icons'

/** A password field with an eye button to show or hide what was typed. The
 * button is skipped by Tab (the field keeps its usual tab order) but works
 * with the mouse and screen readers; the text hides again when unmounted. */
export function PasswordInput({ className, ...props }: Omit<InputHTMLAttributes<HTMLInputElement>, 'type'>) {
  const [shown, setShown] = useState(false)
  return (
    <span className={`password-input${className ? ` ${className}-wrap` : ''}`}>
      <input {...props} className={className} type={shown ? 'text' : 'password'} spellCheck={false} autoCapitalize="off" autoCorrect="off" />
      <button
        type="button"
        className="password-toggle"
        aria-label={shown ? 'Hide password' : 'Show password'}
        aria-pressed={shown}
        title={shown ? 'Hide' : 'Show'}
        tabIndex={-1}
        disabled={props.disabled}
        onMouseDown={(e) => e.preventDefault()} // keep focus (and the caret) in the field
        onClick={() => setShown(!shown)}
      >
        {shown ? <EyeOffIcon /> : <EyeIcon />}
      </button>
    </span>
  )
}
