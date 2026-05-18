using System.Collections;
using System.Collections.Generic;
using UnityEngine;
using UnityEngine.UI;
using TMPro;
using System;

public class SettingsManager : MonoBehaviour
{
    public Slider vol; 
    public void OnValueChanged()
    {
        foreach (AudioSource x in FindObjectsOfType<AudioSource>())
            x.volume = vol.value;
        SceneTransport.Instance.volume = vol.value;
    }

    public TextMeshProUGUI[] buttonTexts;
    public string listenStr;
    bool listen;
    int akt;
    public void Click(int buttonIndx)
    {
        SoundManager.Instance.Sfx(SoundManager.Instance.click);
        if (!listen)
        {
            listen = true;
            akt = buttonIndx;
            buttonTexts[akt].text = listenStr;
            return;
        }

        if (buttonIndx == akt)
        {
            ShowDef(akt);
            listen = false;
        }
        else
        {
            ShowDef(akt);
            akt = buttonIndx;
            buttonTexts[akt].text = listenStr;
        }
    }

    private void Start()
    {
        vol.value = SceneTransport.Instance.volume;
        for (int i = 0; i < buttonTexts.Length; i++)
            ShowDef(i);
    }

    private void Update()
    {
        if (!listen) return;

        foreach(KeyCode k in Enum.GetValues(typeof(KeyCode)))
        {
            if(k != KeyCode.Mouse0 && Input.GetKeyDown(k))
            {
                Change(akt, k);
                break;
            }
        }
    }

    void ShowDef(int x)
    {
        buttonTexts[x].text = SceneTransport.Instance.binds[x].ToString();
    }

    void Change(int x, KeyCode k)
    {
        SceneTransport.Instance.binds[x] = k;
        ShowDef(x);
        listen = false;
    }
}
