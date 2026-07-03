import { useEffect } from 'react'
import './Common.css'

/**
 * Alert component - displays success/error/info messages
 * Auto-dismisses after 5 seconds or when close button clicked
 * 
 * @param {string} type - 'success' | 'error' | 'info'
 * @param {string} message - The message to display
 * @param {function} onClose - Called when alert closes
 * @param {string} download - Optional: data to download as file
 */
export default function Alert({ type, message, onClose, download }) {
  // Auto-close after 5 seconds
  useEffect(() => {
    const timer = setTimeout(onClose, 5000)
    return () => clearTimeout(timer) // Cleanup if component unmounts
  }, [onClose])

  return (
    <div className={`alert alert-${type}`}>
      <div className="alert-content">
        <span>{message}</span>
        
        {/* Optional download link for files (like private keys) */}
        {download && (
          <a 
            href={`data:text/plain,${encodeURIComponent(download)}`} 
            download="private-key.txt"
            className="download-link"
          >
            Download
          </a>
        )}
      </div>
      
      <button onClick={onClose} className="alert-close" aria-label="Close alert">
        ×
      </button>
    </div>
  )
}