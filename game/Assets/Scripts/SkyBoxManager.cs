using System.Collections;
using System.Collections.Generic;
using UnityEngine;

public class SkyBoxManager : MonoBehaviour
{
    public Material mat;
    public Material assetMaterial;

    Vector3 desiredColor;
    Vector3 startColor = new Vector3(0.5f, 0.5f, 0.5f), startColor2 = new Vector3(0.5f, 0.5f, 0.5f);
    bool isChanging = false;
    float percent = 0;
    public float speed;
    float startSpeed;
    bool pulse = false, byHand = false;

    public Color[] colors;

    public Color red;

    public static SkyBoxManager Instance { get; private set; }

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
    }

    private void OnDestroy()
    {
        if (Instance == this) Instance = null;
    }

    private void Start()
    {
        startSpeed = speed;
        mat = new Material(assetMaterial);
        RenderSettings.skybox = mat;
    }

    private void Update()
    {
        if (isChanging)
        {
            percent += speed * Time.deltaTime;
            Vector3 aktColor = Vector3.Lerp(startColor, desiredColor, percent);
            mat.SetColor("_TintColor", new Color(aktColor.x, aktColor.y, aktColor.z, 1));
            if (percent >= 1)
            {
                isChanging = false;
                if (pulse)
                    ChangeSky(new Color(startColor2.x, startColor2.y, startColor2.z, 1), false, speed, byHand, false);
            }
        }
        else
        {
            ChangeSky(colors[Random.Range(0, colors.Length)], false, startSpeed, false, false);
        }
    }

    public void ChangeSky(Color color, bool _pulse, float _speed, bool _byHand, bool brighter)
    {
        speed = _speed;
        pulse = _pulse;
        percent = 0;
        Color c = mat.GetColor("_TintColor");
        
        startColor = new Vector3(c.r, c.g, c.b);
        if (!pulse || !isChanging || !byHand) startColor2 = startColor;
        isChanging = true;
        byHand = _byHand;

        if (!brighter)
            desiredColor = new Vector3(color.r, color.g, color.b);
        else
            desiredColor = (startColor + new Vector3(1, 1, 1)) / 2;
    }
}
