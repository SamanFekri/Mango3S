import { useState } from 'react'
import licenseText from '../../../LICENSE?raw'
import appIcon from '../assets/appicon.png'
import thirdParty from '../assets/third-party.json'
import { useApp } from '../state/AppContext'
import { Modal } from './common'

const COPYRIGHT = 'Copyright © 2026 saman'

interface Component {
  kind: 'go' | 'js'
  name: string
  version: string
  license: string
}

/** About Mango3S: the version, and the license (GPL-3.0) with third-party notices. */
export function AboutDialog({ onClose }: { onClose: () => void }) {
  const { appInfo: i } = useApp()
  const [tab, setTab] = useState<'about' | 'license'>('about')
  const built = i?.buildTime ? new Date(i.buildTime) : null
  return (
    <Modal
      title="About Mango3S"
      onClose={onClose}
      footer={
        <>
          <span className="spacer" />
          <button className="primary" onClick={onClose} autoFocus>
            Close
          </button>
        </>
      }
    >
      <div className="about">
        <div className="about-tabs" role="tablist" aria-label="About Mango3S">
          <button role="tab" id="about-tab-about" aria-selected={tab === 'about'} aria-controls="about-panel" className={tab === 'about' ? 'active' : ''} onClick={() => setTab('about')}>
            About
          </button>
          <button role="tab" id="about-tab-license" aria-selected={tab === 'license'} aria-controls="about-panel" className={tab === 'license' ? 'active' : ''} onClick={() => setTab('license')}>
            License
          </button>
        </div>

        <div id="about-panel" role="tabpanel" aria-labelledby={`about-tab-${tab}`}>
          {tab === 'about' ? (
            <div className="about-main">
              <img src={appIcon} alt="" width={84} height={84} />
              <h3>Mango3S</h3>
              <p className="muted">A desktop IDE for MongoDB</p>
              <p className="about-version">
                Version {i?.version ?? '—'}
                {i?.commit && (
                  <span className="muted">
                    {' '}
                    (build {i.commit.slice(0, 7)}
                    {built && !isNaN(built.getTime()) ? `, ${built.toLocaleDateString()}` : ''})
                  </span>
                )}
              </p>
              <p className="small">{COPYRIGHT}</p>
              <p className="muted small about-notice">
                This program is free software: you can redistribute it and/or modify it under the terms of the GNU General Public License as published by
                the Free Software Foundation, either version 3 of the License, or (at your option) any later version. It comes with ABSOLUTELY NO
                WARRANTY.{' '}
                <button className="link" onClick={() => setTab('license')}>
                  See the license.
                </button>
              </p>
              <p className="muted small">An independent tool inspired by Studio 3T. MongoDB is a trademark of MongoDB, Inc.</p>
            </div>
          ) : (
            <div className="about-license">
              <p className="small">
                Mango3S is licensed under the <strong>GNU General Public License v3.0</strong> (or later). {COPYRIGHT}.
              </p>
              <pre className="license-text" aria-label="License text" tabIndex={0}>
                {licenseText}
              </pre>
              <details className="about-details">
                <summary>Third-party software ({(thirdParty as Component[]).length})</summary>
                <p className="muted small">Mango3S includes the following open-source components under their own licenses.</p>
                <table className="grid compact third-party">
                  <tbody>
                    {(thirdParty as Component[]).map((c) => (
                      <tr key={`${c.kind}:${c.name}`}>
                        <td className="mono">{c.name}</td>
                        <td className="mono muted">{c.version}</td>
                        <td>{c.license}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </details>
            </div>
          )}
        </div>
      </div>
    </Modal>
  )
}
