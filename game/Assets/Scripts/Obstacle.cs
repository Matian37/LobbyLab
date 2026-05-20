using System.Collections;
using System.Collections.Generic;
using UnityEngine;

public class Obstacle : MonoBehaviour
{
    Transform player;
    public float loseOffset = 2;
    bool done = false;
    public bool isLeft;
    public bool isWall = false, left;
    Material doneMat;
    bool isChanging = false;
    float percent;
    public float speed;
    Texture2D tex;
    public float downForce = 100, downForce2 = 100, sideForce = 1000, forwardForce = 1000, forwardForce2 = 10000;
    Vector3 color1, color2, col, playerPosition;
    public Color skyColor;

    public bool isMultiplayer;
    private void Start()
    {
        foreach (var x in GameObject.FindGameObjectsWithTag("Player"))
        {
            if(x.activeInHierarchy)
            {
                player = x.transform;
                break;
            }
        }
    }

    private void Update()
    {
        if(transform.position.z - loseOffset > player.position.z)
        {
            if (!done)
                Lose();
            Destroy(gameObject);
        }

        if (isChanging)
        {
            percent += speed * Time.deltaTime;
            col = Vector3.Lerp(color1, color2, percent);

            tex.SetPixel(0, 0, new Color(col.x, col.y, col.z));
            tex.Apply();
            doneMat.SetTexture("_Texture", tex);

            if (percent > 1)
                isChanging = false;
        }
    }

    void CompleteObs()
    {
        done = true;
        isChanging = true;
        percent = 0;
        Color c = Color.white;
        Color c1 = SkyBoxManager.Instance.mat.GetColor("_TintColor") * 0.75f + skyColor * 0.25f;
        tex = new Texture2D(1, 1);

        gameObject.AddComponent<Rigidbody>();


        if (isWall && left)
            GetComponent<Rigidbody>().velocity = Vector3.right * sideForce;
        else if (isWall)
            GetComponent<Rigidbody>().velocity = Vector3.left * sideForce;
        else
            GetComponent<Rigidbody>().velocity = Vector3.back * forwardForce;

        doneMat = new Material(Shader.Find("Shader Graphs/Shader"));
        doneMat.SetFloat("_BackwaysStrength", -0.0015f);
        color1 = new Vector3(c.r, c.g, c.b);
        color2 = new Vector3(c1.r, c1.g, c1.b);
        transform.GetChild(0).GetComponent<Renderer>().material = doneMat;
        transform.GetChild(0).GetComponent<Animator>().SetTrigger("Do");
    }

    void Lose()
    {
        if (isMultiplayer) return;
        foreach (var obj in FindObjectsOfType<GameManager>())
        {
            if(obj.gameObject.activeInHierarchy)
            {
                obj.Lose(!isLeft);
            }
        }
    }

    private void OnTriggerEnter(Collider other)
    {
        if (other.tag == "Player")
        {
            if (!done)
            {
                //SkyBoxManager.Instance.ChangeSky(Color.white, true, 15, true, true);
                playerPosition = other.transform.position;
                SceneTransport.Instance.xp++;
                CompleteObs();

                if(isWall && !isMultiplayer)
                    other.GetComponent<PlayerController>().touchedWall = true;
            }
        }
    }
}
