using System.Collections;
using System.Collections.Generic;
using UnityEngine;

public class SoundManager : MonoBehaviour
{
    public static SoundManager Instance { get; private set; }

    public AudioSource source, sourceSfx;
    public AudioClip[] music;
    public AudioClip menuMusic;
    public float timeToMaxVol = 1.5f;
    float tim;

    public AudioClip buyUnlock, click, collect, error, hover, pause, unpause, unlock, unlockModeOn, unlockModeOff;

    public void Sfx(AudioClip x)
    {
        sourceSfx.PlayOneShot(x);
    }

    public void PlaySong(int indx)
    {
        source.clip = music[indx];
        source.Play();
    }

    public void Pause(bool czy)
    {
        if (czy) source.Pause();
        else source.UnPause();
    }

    public void PlayMenuMusic()
    {
        source.clip = menuMusic;
        source.Play();
        source.volume = 0;
    }

    private void Start()
    {
        sourceSfx.volume = SceneTransport.Instance.volume;
    }

    private void Update()
    {
        tim += Time.deltaTime;
        if (tim >= timeToMaxVol || source.volume > SceneTransport.Instance.volume)
        {
            source.volume = SceneTransport.Instance.volume;
            return;
        }
        source.volume += SceneTransport.Instance.volume / timeToMaxVol * Time.deltaTime;
    }

    private void OnDestroy()
    {
        if (Instance == this) Instance = null;
    }

    public void StopMusic()
    {
        source.Stop();
    }

    private void Awake()
    {
        if (Instance is null)
        {
            Instance = this;
        }
        else
        {
            Destroy(gameObject);
        }

        DontDestroyOnLoad(gameObject);
    }
}
